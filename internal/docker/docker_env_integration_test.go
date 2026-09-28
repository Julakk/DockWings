//go:build integration

package docker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Julakk/DockWings/internal/server"
)

const testImage = "alpine:3.20"

func dockerOut(args ...string) (string, error) {
	out, err := exec.Command("docker", args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func isRunning(name string) bool {
	out, err := dockerOut("inspect", "-f", "{{.State.Running}}", name)
	return err == nil && out == "true"
}

func waitFor(t *testing.T, what string, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("timeout nunggu: %s", what)
}

func TestDockerEnvironmentLifecycle(t *testing.T) {
	if _, err := dockerOut("info"); err != nil {
		t.Skipf("docker nggak tersedia: %v", err)
	}

	dataDir := t.TempDir()
	env := NewDockerEnvironment(dataDir)
	ctx := context.Background()

	s := &server.Server{
		UUID:            fmt.Sprintf("itest-%d", time.Now().UnixNano()),
		Image:           testImage,
		StartupCommand:  "cat", // echo stdin ke stdout, kelihatan di docker logs
		MemoryLimitMB:   64,
		CPULimitPercent: 50,
		EnvVariables:    map[string]string{"FOO": "bar"},
	}
	name := s.ContainerName()
	t.Cleanup(func() { _, _ = dockerOut("rm", "-f", name) })

	// Create
	if err := env.Create(ctx, s); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if s.Status != server.StatusOffline {
		t.Errorf("setelah Create status = %q, mau offline", s.Status)
	}
	if _, err := dockerOut("inspect", name); err != nil {
		t.Fatalf("container nggak ada setelah Create: %v", err)
	}
	if isRunning(name) {
		t.Fatal("container jalan padahal baru Create")
	}

	// Start
	if err := env.Start(ctx, s); err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitFor(t, "container running", 10*time.Second, func() bool { return isRunning(name) })

	// Env variable masuk ke container
	if out, err := dockerOut("exec", name, "printenv", "FOO"); err != nil || out != "bar" {
		t.Errorf("env FOO = %q (err=%v), mau %q", out, err, "bar")
	}

	// SendCommand -> stdin -> cat -> logs
	if err := env.SendCommand(ctx, s, "hello-dockwings"); err != nil {
		t.Fatalf("SendCommand: %v", err)
	}
	waitFor(t, "command muncul di logs", 10*time.Second, func() bool {
		out, _ := dockerOut("logs", name)
		return strings.Contains(out, "hello-dockwings")
	})

	// Stop (graceful). Durasinya dicatat karena PID 1 bisa ngabaikan SIGTERM.
	t0 := time.Now()
	if err := env.Stop(ctx, s); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	t.Logf("Stop makan waktu %s", time.Since(t0).Round(time.Millisecond))
	if s.Status != server.StatusOffline || isRunning(name) {
		t.Errorf("setelah Stop: status=%q running=%v", s.Status, isRunning(name))
	}

	// Start lagi lalu Kill
	if err := env.Start(ctx, s); err != nil {
		t.Fatalf("Start ke-2: %v", err)
	}
	waitFor(t, "running lagi", 10*time.Second, func() bool { return isRunning(name) })
	if err := env.Kill(ctx, s); err != nil {
		t.Fatalf("Kill: %v", err)
	}
	waitFor(t, "container mati setelah Kill", 10*time.Second, func() bool { return !isRunning(name) })

	// Remove: container hilang, folder data tetap ada, dan idempotent
	if err := env.Remove(ctx, s); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := dockerOut("inspect", name); err == nil {
		t.Error("container masih ada setelah Remove")
	}
	if _, err := os.Stat(filepath.Join(dataDir, s.UUID)); err != nil {
		t.Errorf("folder data harusnya tetap ada: %v", err)
	}
	if err := env.Remove(ctx, s); err != nil {
		t.Errorf("Remove kedua harusnya nggak error: %v", err)
	}
}
