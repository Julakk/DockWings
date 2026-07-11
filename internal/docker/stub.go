package docker

import (
	"context"
	"log"

	"github.com/Julakk/DockWings/internal/server"
)

// StubEnvironment adalah implementasi Environment yang cuma nge-log doang,
// nggak beneran manggil Docker. Dipakai sebagai default sampai daemon ini
// dijalanin di VPS beneran dengan Docker terpasang.
//
// Ganti ke DockerEnvironment (belum dibuat) begitu development lanjut ke tahap
// integrasi Docker asli.
type StubEnvironment struct{}

func NewStubEnvironment() *StubEnvironment {
	return &StubEnvironment{}
}

func (e *StubEnvironment) Create(ctx context.Context, s *server.Server) error {
	log.Printf("[stub] create container buat server %s (image: %s)", s.UUID, s.Image)
	return nil
}

func (e *StubEnvironment) Start(ctx context.Context, s *server.Server) error {
	log.Printf("[stub] start container %s", s.ContainerName())
	return nil
}

func (e *StubEnvironment) Stop(ctx context.Context, s *server.Server) error {
	log.Printf("[stub] stop container %s", s.ContainerName())
	return nil
}

func (e *StubEnvironment) Kill(ctx context.Context, s *server.Server) error {
	log.Printf("[stub] kill container %s", s.ContainerName())
	return nil
}

func (e *StubEnvironment) Remove(ctx context.Context, s *server.Server) error {
	log.Printf("[stub] remove container %s", s.ContainerName())
	return nil
}

func (e *StubEnvironment) SendCommand(ctx context.Context, s *server.Server, command string) error {
	log.Printf("[stub] kirim command ke %s: %s", s.ContainerName(), command)
	return nil
}
