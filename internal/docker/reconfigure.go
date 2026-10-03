package docker

import (
	"context"
	"fmt"
	"time"

	"github.com/Julakk/DockWings/internal/server"
)

// Reconfigurer dipenuhi environment yang bisa bikin ulang container dengan
// konfigurasi terbaru (mis. port allocation berubah). Sengaja interface
// terpisah supaya Environment dan StubEnvironment nggak perlu ikut berubah.
type Reconfigurer interface {
	// Reconfigure bikin ulang container dari data server saat ini.
	// restarted = true kalau container sebelumnya jalan dan sudah dinyalakan lagi.
	Reconfigure(ctx context.Context, s *server.Server) (restarted bool, err error)
}

var _ Reconfigurer = (*DockerEnvironment)(nil)

// Reconfigure: Docker nggak bisa ganti port mapping container yang sudah ada,
// jadi container dibuat ulang. Folder data (bind mount) tetap utuh. Container
// lama di-rename dulu sebagai cadangan, jadi kalau create gagal bisa dikembalikan.
func (e *DockerEnvironment) Reconfigure(_ context.Context, s *server.Server) (bool, error) {
	// Context sendiri, biar proses tetap selesai walau Panel udah timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	name := s.ContainerName()
	stateOut, err := run(ctx, "inspect", "-f", "{{.State.Status}}", name)
	if err != nil {
		// Belum ada container: tidak ada yang perlu dibuat ulang,
		// allocation baru dipakai pas container dibuat (Provision).
		return false, nil
	}

	state := mapDockerState(stateOut)
	wasRunning := state == "running" || state == "starting"

	if wasRunning {
		if _, err := run(ctx, "stop", "-t", "30", name); err != nil {
			return false, err
		}
	}

	backup := name + "-old"
	_, _ = run(ctx, "rm", "-f", backup)
	if _, err := run(ctx, "rename", name, backup); err != nil {
		return false, err
	}

	if err := e.create(s, false); err != nil {
		_, _ = run(ctx, "rm", "-f", name)
		_, _ = run(ctx, "rename", backup, name)
		if wasRunning {
			_, _ = run(ctx, "start", name)
		}
		return false, fmt.Errorf("gagal bikin ulang container, container lama dikembalikan: %w", err)
	}
	_, _ = run(ctx, "rm", "-f", backup)

	if !wasRunning {
		return false, nil
	}
	if err := e.Start(ctx, s); err != nil {
		return false, fmt.Errorf("container dibuat ulang tapi gagal start (port bentrok?): %w", err)
	}

	return true, nil
}
