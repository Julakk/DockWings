package server

import "fmt"

// Update ngubah satu server lewat fn (di bawah lock) lalu nyimpen state ke disk.
func (m *Manager) Update(uuid string, fn func(*Server)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.servers[uuid]
	if !ok {
		return fmt.Errorf("server %s nggak ketemu di daemon ini", uuid)
	}

	fn(s)
	m.save()

	return nil
}
