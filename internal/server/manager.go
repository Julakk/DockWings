package server

import (
	"fmt"
	"sync"
)

// Manager nyimpen semua server yang lagi dikelola daemon ini, key-nya UUID server.
// Data ini cuma di memory buat sekarang — belum ada persistence ke disk.
// TODO: persist ke file/database lokal biar survive restart daemon.
type Manager struct {
	mu      sync.RWMutex
	servers map[string]*Server
}

func NewManager() *Manager {
	return &Manager{
		servers: make(map[string]*Server),
	}
}

func (m *Manager) Add(s *Server) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.servers[s.UUID] = s
}

func (m *Manager) Get(uuid string) (*Server, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, ok := m.servers[uuid]
	if !ok {
		return nil, fmt.Errorf("server %s nggak ketemu di daemon ini", uuid)
	}

	return s, nil
}

func (m *Manager) Remove(uuid string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.servers, uuid)
}

func (m *Manager) All() []*Server {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]*Server, 0, len(m.servers))
	for _, s := range m.servers {
		list = append(list, s)
	}

	return list
}
