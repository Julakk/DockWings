package docker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Julakk/DockWings/internal/server"
)

// DockerEnvironment ngontrol container lewat Docker CLI.
type DockerEnvironment struct {
	dataDir string
}

func NewDockerEnvironment(dataDir string) *DockerEnvironment {
	return &DockerEnvironment{dataDir: dataDir}
}

func run(ctx context.Context, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		return text, fmt.Errorf("docker %s gagal: %v: %s", args[0], err, text)
	}
	return text, nil
}

func (e *DockerEnvironment) Create(_ context.Context, s *server.Server) error {
	return e.create(s, true)
}

// create bikin container. pull=false dipakai Reconfigure (image sudah ada lokal).
func (e *DockerEnvironment) create(s *server.Server, pull bool) error {
	// Context sendiri, biar pull image tetap jalan walau Panel udah timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	if s.Image == "" {
		s.Status = "error"
		return fmt.Errorf("image kosong")
	}

	dir := filepath.Join(e.dataDir, s.UUID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		s.Status = "error"
		return err
	}

	if pull {
		if _, err := run(ctx, "pull", s.Image); err != nil && !imageLocal(ctx, s.Image) {
			s.Status = "error"
			return err
		}
	}

	// Hapus container lama dengan nama sama (kalau re-provision).
	_, _ = run(ctx, "rm", "-f", s.ContainerName())

	args := []string{
		"create", "--name", s.ContainerName(),
		"-i", // stdin tetap terbuka, dipakai SendCommand
		"-v", dir + ":/home/container",
		"-w", "/home/container",
		"--cap-drop=ALL",
		"--security-opt=no-new-privileges",
		"--pids-limit=512",
	}
	if s.MemoryLimitMB > 0 {
		args = append(args, fmt.Sprintf("--memory=%dm", s.MemoryLimitMB))
		if s.SwapMB < 0 {
			args = append(args, "--memory-swap=-1")
		} else {
			args = append(args, fmt.Sprintf("--memory-swap=%dm", s.MemoryLimitMB+s.SwapMB))
		}
	}
	if s.CPULimitPercent > 0 {
		args = append(args, fmt.Sprintf("--cpus=%.2f", s.CPULimitPercent/100))
	}
	if s.IOWeight >= 10 && s.IOWeight <= 1000 {
		args = append(args, fmt.Sprintf("--blkio-weight=%d", s.IOWeight))
	}

	// Mapping port: 1:1 host->container (ngikutin konvensi Pterodactyl,
	// nggak ada remapping). SERVER_IP/SERVER_PORT di-inject dari allocation
	// primary biar startup command / app di dalam container tau mau bind ke mana.
	for _, a := range s.Allocations {
		bindIP := a.IP
		if bindIP == "" {
			bindIP = "0.0.0.0"
		}
		args = append(args, "-p", fmt.Sprintf("%s:%d:%d/tcp", bindIP, a.Port, a.Port))
		args = append(args, "-p", fmt.Sprintf("%s:%d:%d/udp", bindIP, a.Port, a.Port))
	}
	if primary := s.PrimaryAllocation(); primary != nil {
		serverIP := primary.IP
		if serverIP == "" {
			serverIP = "0.0.0.0"
		}
		args = append(args, "-e", "SERVER_IP="+serverIP)
		args = append(args, "-e", fmt.Sprintf("SERVER_PORT=%d", primary.Port))
	}
	for k, v := range s.EnvVariables {
		args = append(args, "-e", k+"="+v)
	}
	if s.StartupCommand != "" {
		args = append(args, "--entrypoint", "/bin/sh", s.Image, "-c", s.StartupCommand)
	} else {
		args = append(args, s.Image)
	}

	if _, err := run(ctx, args...); err != nil {
		s.Status = "error"
		return err
	}

	s.Status = server.StatusOffline
	return nil
}

func (e *DockerEnvironment) Start(ctx context.Context, s *server.Server) error {
	// Pernah dinyalakan = nyala lagi otomatis setelah reboot host, kecuali di-Stop/Kill manual.
	// Dipasang di sini (bukan di Create) biar server yang belum pernah di-Start nggak ikut jalan sendiri.
	_, _ = run(ctx, "update", "--restart=unless-stopped", s.ContainerName())
	if _, err := run(ctx, "start", s.ContainerName()); err != nil {
		s.Status = server.StatusOffline
		return err
	}
	s.Status = server.StatusRunning
	return nil
}

func (e *DockerEnvironment) Stop(ctx context.Context, s *server.Server) error {
	if _, err := run(ctx, "stop", "-t", "30", s.ContainerName()); err != nil {
		return err
	}
	s.Status = server.StatusOffline
	return nil
}

func (e *DockerEnvironment) Kill(ctx context.Context, s *server.Server) error {
	if _, err := run(ctx, "kill", s.ContainerName()); err != nil {
		return err
	}
	s.Status = server.StatusOffline
	return nil
}

// Remove hapus container. Folder data server sengaja TIDAK dihapus.
func (e *DockerEnvironment) Remove(ctx context.Context, s *server.Server) error {
	if out, err := run(ctx, "rm", "-f", s.ContainerName()); err != nil &&
		!strings.Contains(out, "No such container") {
		return err
	}
	return nil
}

// SendCommand nulis ke stdin proses utama (PID 1) container.
func (e *DockerEnvironment) SendCommand(ctx context.Context, s *server.Server, command string) error {
	_, err := run(ctx, "exec", s.ContainerName(), "sh", "-c",
		`printf '%s\n' "$1" > /proc/1/fd/0`, "sh", command)
	return err
}
