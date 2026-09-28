package docker

import (
	"context"

	"github.com/Julakk/DockWings/internal/server"
)

// Environment adalah abstraksi buat operasi container.
// Sengaja dibikin interface, bukan langsung manggil Docker SDK,
// biar:
//  1. Logic API/handler bisa ditulis & ditest sekarang (pakai StubEnvironment)
//  2. Nanti gampang diganti DockerEnvironment asli begitu ada VPS buat testing,
//     tanpa perlu ubah kode di internal/api sama sekali.
type Environment interface {
	// Create bikin container baru sesuai spec server (belum di-start).
	Create(ctx context.Context, s *server.Server) error

	// Start nyalain container yang udah ada.
	Start(ctx context.Context, s *server.Server) error

	// Stop ngirim sinyal stop (graceful) ke container.
	Stop(ctx context.Context, s *server.Server) error

	// Kill paksa matiin container (SIGKILL / force remove).
	Kill(ctx context.Context, s *server.Server) error

	// Remove ngehapus container dari Docker sepenuhnya.
	Remove(ctx context.Context, s *server.Server) error

	// SendCommand ngirim command ke stdin container (console command kayak "say halo").
	SendCommand(ctx context.Context, s *server.Server, command string) error

	// Resources ambil state + utilization CPU/mem/disk terkini.
	Resources(ctx context.Context, s *server.Server) (Resources, error)
}
