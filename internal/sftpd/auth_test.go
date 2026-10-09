package sftpd

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPanelAuth(t *testing.T) {
	var gotAuth, gotUser string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		var in map[string]string
		_ = json.NewDecoder(r.Body).Decode(&in)
		gotUser = in["username"]
		if in["password"] != "benar" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"x"}`))
			return
		}
		_, _ = w.Write([]byte(`{"server":"` + testUUID + `","read_only":true}`))
	}))
	defer ts.Close()

	auth := NewPanelAuth(ts.URL+"/", "tok")
	res, err := auth(context.Background(), "u.abcd1234", "benar", "1.2.3.4")
	if err != nil {
		t.Fatal(err)
	}
	if res.ServerUUID != testUUID || !res.ReadOnly {
		t.Fatalf("hasil salah: %+v", res)
	}
	if gotAuth != "Bearer tok" || gotUser != "u.abcd1234" {
		t.Fatalf("request salah: %q %q", gotAuth, gotUser)
	}
	if _, err := auth(context.Background(), "u.abcd1234", "salah", "1.2.3.4"); !errors.Is(err, ErrAuthDenied) {
		t.Fatalf("harusnya ErrAuthDenied, dapat %v", err)
	}
}
