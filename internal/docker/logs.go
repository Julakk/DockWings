package docker

import (
	"context"
	"io"
	"os/exec"

	"github.com/Julakk/DockWings/internal/server"
)

// LogStreamer opsional buat implementasi Environment yang support streaming
// log real-time. StubEnvironment sengaja nggak implement ini.
type LogStreamer interface {
	StreamLogs(ctx context.Context, s *server.Server) (io.ReadCloser, error)
}

type logStream struct {
	rc  io.ReadCloser
	cmd *exec.Cmd
}

func (l *logStream) Read(p []byte) (int, error) { return l.rc.Read(p) }

func (l *logStream) Close() error {
	if l.cmd.Process != nil {
		_ = l.cmd.Process.Kill()
	}
	return l.rc.Close()
}

// StreamLogs nge-tail log container secara live lewat `docker logs -f`.
func (e *DockerEnvironment) StreamLogs(ctx context.Context, s *server.Server) (io.ReadCloser, error) {
	cmd := exec.CommandContext(ctx, "docker", "logs", "-f", "--tail", "100", s.ContainerName())

	pr, pw := io.Pipe()
	cmd.Stdout = pw
	cmd.Stderr = pw

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	go func() {
		_ = cmd.Wait()
		_ = pw.Close()
	}()

	return &logStream{rc: pr, cmd: cmd}, nil
}
