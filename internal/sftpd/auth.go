package sftpd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ErrAuthDenied = Panel menolak login (username/password salah, server suspended, dst).
var ErrAuthDenied = errors.New("login ditolak")

// AuthResult hasil verifikasi login: server mana yang boleh diakses dan apakah read-only.
type AuthResult struct {
	ServerUUID string
	ReadOnly   bool
}

type AuthFunc func(ctx context.Context, username, password, ip string) (*AuthResult, error)

// NewPanelAuth bikin AuthFunc yang nanya ke Panel (POST /api/remote/sftp/auth,
// auth Bearer daemon_token). panelURL sebaiknya https supaya POST nggak
// berubah jadi GET lewat redirect.
func NewPanelAuth(panelURL, token string) AuthFunc {
	endpoint := strings.TrimRight(panelURL, "/") + "/api/remote/sftp/auth"
	client := &http.Client{Timeout: 10 * time.Second}

	return func(ctx context.Context, username, password, ip string) (*AuthResult, error) {
		body, err := json.Marshal(map[string]string{"username": username, "password": password, "ip": ip})
		if err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("panel nggak bisa dihubungi: %w", err)
		}
		defer resp.Body.Close()

		switch resp.StatusCode {
		case http.StatusOK:
		case http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusTooManyRequests:
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
			return nil, ErrAuthDenied
		default:
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
			return nil, fmt.Errorf("panel balas HTTP %d", resp.StatusCode)
		}

		var out struct {
			Server   string `json:"server"`
			ReadOnly bool   `json:"read_only"`
		}
		if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&out); err != nil {
			return nil, fmt.Errorf("balasan panel nggak valid: %w", err)
		}
		if !uuidRe.MatchString(out.Server) {
			return nil, errors.New("balasan panel: uuid server nggak valid")
		}
		return &AuthResult{ServerUUID: out.Server, ReadOnly: out.ReadOnly}, nil
	}
}
