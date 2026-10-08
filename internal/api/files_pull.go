package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"syscall"
	"time"

	"github.com/Julakk/DockWings/internal/files"
)

var (
	errBlockedAddr = errors.New("alamat tujuan nggak diizinkan (hanya IP publik)")
	errBadURL      = errors.New("URL nggak valid (harus http atau https)")
	errPullTooBig  = errors.New("file kegedean (maks 100 MB)")
)

var blockedNets = mustCIDRs(
	"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24",
	"198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4",
	"64:ff9b::/96", "fc00::/7", "fe80::/10",
)

func mustCIDRs(list ...string) []*net.IPNet {
	out := make([]*net.IPNet, 0, len(list))
	for _, c := range list {
		_, n, err := net.ParseCIDR(c)
		if err != nil {
			panic(err)
		}
		out = append(out, n)
	}
	return out
}

// publicIP true cuma kalau ip itu alamat publik biasa.
func publicIP(ip net.IP) bool {
	if ip == nil || ip.IsUnspecified() || ip.IsLoopback() || ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsMulticast() {
		return false
	}
	for _, n := range blockedNets {
		if n.Contains(ip) {
			return false
		}
	}
	return true
}

func ownIP(ip net.IP) bool {
	addrs, _ := net.InterfaceAddrs()
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok && n.IP.Equal(ip) {
			return true
		}
	}
	return false
}

// guardControl dipanggil Go SETELAH DNS di-resolve, tepat sebelum connect,
// jadi DNS rebinding dan redirect ke IP internal ikut tertolak.
func guardControl(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return errBlockedAddr
	}
	ip := net.ParseIP(host)
	if !publicIP(ip) || ownIP(ip) {
		return errBlockedAddr
	}
	return nil
}

func newPullClient() *http.Client {
	d := &net.Dialer{Timeout: 15 * time.Second, Control: guardControl}
	return &http.Client{
		Transport: &http.Transport{
			Proxy:                 nil,
			DialContext:           d.DialContext,
			TLSHandshakeTimeout:   15 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
			DisableKeepAlives:     true,
		},
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("redirect kebanyakan")
			}
			return nil
		},
	}
}

type pullNetError struct{ err error }

func (e *pullNetError) Error() string { return "gagal download: " + e.err.Error() }
func (e *pullNetError) Unwrap() error { return e.err }

type pullStatusError struct{ code int }

func (e *pullStatusError) Error() string { return fmt.Sprintf("server tujuan balas HTTP %d", e.code) }

// capReader nolak isi lebih dari left byte, tanpa nyisain file setengah jadi
// (Store.Write nulis ke temp file dan menghapusnya kalau error).
type capReader struct {
	r    io.Reader
	left int64
}

func (c *capReader) Read(p []byte) (int, error) {
	if int64(len(p)) > c.left+1 {
		p = p[:c.left+1]
	}
	n, err := c.r.Read(p)
	if int64(n) > c.left {
		n = int(c.left)
		c.left = 0
		return n, errPullTooBig
	}
	c.left -= int64(n)
	if err != nil && err != io.EOF {
		err = &pullNetError{err}
	}
	return n, err
}

func pullToStore(ctx context.Context, st *files.Store, rawURL, dest string, max int64) (int64, error) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return 0, errBadURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return 0, errBadURL
	}
	req.Header.Set("User-Agent", "DockWings")
	resp, err := newPullClient().Do(req)
	if err != nil {
		return 0, &pullNetError{err}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, &pullStatusError{resp.StatusCode}
	}
	if resp.ContentLength > max {
		return 0, errPullTooBig
	}
	return st.Write(dest, &capReader{r: resp.Body, left: max})
}

// POST /api/servers/{uuid}/files/pull  {"url":"https://.../a.zip","path":"/a.zip"}
func (f *FilesHandlers) Pull(w http.ResponseWriter, r *http.Request) {
	st, ok := f.store(w, r)
	if !ok {
		return
	}
	var req struct {
		URL  string `json:"url"`
		Path string `json:"path"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Minute)
	defer cancel()

	n, err := pullToStore(ctx, st, req.URL, req.Path, maxUploadBytes)
	if err != nil {
		var pse *pullStatusError
		var pne *pullNetError
		switch {
		case errors.Is(err, errBlockedAddr), errors.Is(err, errBadURL):
			writeError(w, http.StatusUnprocessableEntity, err.Error())
		case errors.Is(err, errPullTooBig):
			writeError(w, http.StatusRequestEntityTooLarge, err.Error())
		case errors.As(err, &pse):
			writeError(w, http.StatusBadGateway, pse.Error())
		case errors.As(err, &pne):
			log.Printf("pull gagal: %v", err)
			writeError(w, http.StatusBadGateway, "gagal download dari URL tujuan")
		default:
			writeFileError(w, err)
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "size": n})
}
