package server

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// Manager nyimpen semua server yang lagi dikelola daemon ini, key-nya UUID server.
// Kalau path diisi (lewat NewPersistentManager), daftar server disimpan ke disk
// dan dimuat lagi pas daemon restart.
type Manager struct {
	mu      sync.RWMutex
	servers map[string]*Server
	path    string
}

func NewManager() *Manager {
	return &Manager{
		servers: make(map[string]*Server),
	}
}

// NewPersistentManager sama kayak NewManager tapi state disimpan di file JSON.
func NewPersistentManager(path string) *Manager {
	m := NewManager()
	m.path = path
	if err := m.load(); err != nil {
		log.Printf("gagal load state dari %s: %v", path, err)
	}
	return m
}

func (m *Manager) load() error {
	data, err := os.ReadFile(m.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	var list []*Server
	if err := json.Unmarshal(data, &list); err != nil {
		return err
	}

	for _, s := range list {
		// State container nggak diketahui setelah restart, anggap offline.
		s.Status = StatusOffline
		m.servers[s.UUID] = s
	}
	log.Printf("state dimuat: %d server", len(list))
	return nil
}

// save dipanggil saat lock sudah dipegang.
func (m *Manager) save() {
	if m.path == "" {
		return
	}

	list := make([]*Server, 0, len(m.servers))
	for _, s := range m.servers {
		list = append(list, s)
	}

	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		log.Printf("gagal encode state: %v", err)
		return
	}

	if err := os.MkdirAll(filepath.Dir(m.path), 0o755); err != nil {
		log.Printf("gagal bikin folder state: %v", err)
		return
	}

	tmp := m.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		log.Printf("gagal tulis state: %v", err)
		return
	}
	if err := os.Rename(tmp, m.path); err != nil {
		log.Printf("gagal rename state: %v", err)
	}
}

func (m *Manager) Add(s *Server) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.servers[s.UUID] = s
	m.save()
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
	m.save()
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
