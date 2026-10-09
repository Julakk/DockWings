// Package sftpd = server SFTP bawaan DockWings. Login diverifikasi ke Panel,
// dan tiap user dikurung di folder servernya sendiri.
package sftpd

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"github.com/Julakk/DockWings/internal/files"
)

var uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type Config struct {
	DataRoot string // data_directory (isinya folder per uuid server)
	HostKey  ssh.Signer
	Auth     AuthFunc
	MaxConns int // 0 = 128
}

type Server struct {
	cfg    Config
	sshCfg *ssh.ServerConfig
	sem    chan struct{}

	mu     sync.Mutex
	ln     net.Listener
	conns  map[net.Conn]struct{}
	closed bool
	wg     sync.WaitGroup
}

func New(cfg Config) *Server {
	if cfg.MaxConns <= 0 {
		cfg.MaxConns = 128
	}
	s := &Server{
		cfg:   cfg,
		sem:   make(chan struct{}, cfg.MaxConns),
		conns: map[net.Conn]struct{}{},
	}
	s.sshCfg = &ssh.ServerConfig{
		MaxAuthTries:     5,
		ServerVersion:    "SSH-2.0-DockWings",
		PasswordCallback: s.passwordAuth,
	}
	s.sshCfg.AddHostKey(cfg.HostKey)
	return s
}

func ipOf(a net.Addr) string {
	h, _, err := net.SplitHostPort(a.String())
	if err != nil {
		return ""
	}
	return h
}

func (s *Server) passwordAuth(c ssh.ConnMetadata, pw []byte) (*ssh.Permissions, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	res, err := s.cfg.Auth(ctx, c.User(), string(pw), ipOf(c.RemoteAddr()))
	if err != nil {
		if !errors.Is(err, ErrAuthDenied) {
			log.Printf("sftp: verifikasi login gagal: %v", err)
		}
		return nil, errors.New("login ditolak")
	}
	ro := "0"
	if res.ReadOnly {
		ro = "1"
	}
	return &ssh.Permissions{Extensions: map[string]string{"server": res.ServerUUID, "ro": ro}}, nil
}

func (s *Server) ListenAndServe(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return s.Serve(ln)
}

func (s *Server) Serve(ln net.Listener) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		ln.Close()
		return nil
	}
	s.ln = ln
	s.mu.Unlock()

	for {
		c, err := ln.Accept()
		if err != nil {
			s.mu.Lock()
			closed := s.closed
			s.mu.Unlock()
			if closed {
				return nil
			}
			log.Printf("sftp: accept gagal: %v", err)
			time.Sleep(200 * time.Millisecond)
			continue
		}
		select {
		case s.sem <- struct{}{}:
		default:
			c.Close() // koneksi kebanyakan
			continue
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer func() { <-s.sem }()
			s.handle(c)
		}()
	}
}

func (s *Server) Close() {
	s.mu.Lock()
	s.closed = true
	if s.ln != nil {
		s.ln.Close()
	}
	for c := range s.conns {
		c.Close()
	}
	s.mu.Unlock()
	s.wg.Wait()
}

func (s *Server) track(c net.Conn, add bool) {
	s.mu.Lock()
	if add {
		s.conns[c] = struct{}{}
	} else {
		delete(s.conns, c)
	}
	s.mu.Unlock()
}

func (s *Server) handle(nc net.Conn) {
	defer nc.Close()
	s.track(nc, true)
	defer s.track(nc, false)

	_ = nc.SetDeadline(time.Now().Add(30 * time.Second)) // batas waktu handshake + login
	sconn, chans, reqs, err := ssh.NewServerConn(nc, s.sshCfg)
	if err != nil {
		return
	}
	_ = nc.SetDeadline(time.Time{})
	defer sconn.Close()
	go ssh.DiscardRequests(reqs)

	uuid := sconn.Permissions.Extensions["server"]
	ro := sconn.Permissions.Extensions["ro"] == "1"

	for nch := range chans {
		if nch.ChannelType() != "session" {
			_ = nch.Reject(ssh.UnknownChannelType, "hanya sesi SFTP")
			continue
		}
		ch, creqs, err := nch.Accept()
		if err != nil {
			continue
		}
		go s.session(ch, creqs, uuid, ro)
	}
}

// session cuma melayani subsystem "sftp". Shell, exec, pty, dan env ditolak.
func (s *Server) session(ch ssh.Channel, reqs <-chan *ssh.Request, uuid string, ro bool) {
	defer ch.Close()
	started := make(chan struct{}, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for req := range reqs {
			ok := false
			if req.Type == "subsystem" {
				var p struct{ Name string }
				if ssh.Unmarshal(req.Payload, &p) == nil && p.Name == "sftp" {
					ok = true
					select {
					case started <- struct{}{}:
					default:
					}
				}
			}
			if req.WantReply {
				_ = req.Reply(ok, nil)
			}
		}
	}()
	select {
	case <-started:
		s.serveSFTP(ch, uuid, ro)
	case <-done:
	case <-time.After(30 * time.Second):
	}
}

func (s *Server) serveSFTP(ch ssh.Channel, uuid string, ro bool) {
	if !uuidRe.MatchString(uuid) {
		return
	}
	st := files.New(s.cfg.DataRoot, uuid)
	if fi, err := os.Stat(st.Base()); err != nil || !fi.IsDir() {
		return // server belum di-provision di node ini
	}
	srv := sftp.NewRequestServer(ch, newHandlers(st, ro))
	if err := srv.Serve(); err != nil && !errors.Is(err, io.EOF) {
		log.Printf("sftp: sesi berakhir: %v", err)
	}
	_ = srv.Close()
}
