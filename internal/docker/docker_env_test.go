package docker

import (
	"context"
	"testing"

	"github.com/Julakk/DockWings/internal/server"
)

// Compile-time check: DockerEnvironment harus memenuhi interface Environment.
var _ Environment = (*DockerEnvironment)(nil)

func TestCreateEmptyImage(t *testing.T) {
	env := NewDockerEnvironment(t.TempDir())
	s := &server.Server{UUID: "x"}

	if err := env.Create(context.Background(), s); err == nil {
		t.Fatal("expected error untuk image kosong, tapi nil")
	}
	if s.Status != "error" {
		t.Errorf("status = %q, mau %q", s.Status, "error")
	}
}
