package store

import "sync"

// Mem is an in-memory KV map (slice 0). Later slices replace this with pages.
type Mem struct {
	mu   sync.RWMutex
	data map[string][]byte
}

func NewMem() *Mem {
	return &Mem{data: make(map[string][]byte)}
}

func (m *Mem) Put(key string, value []byte) {
	cp := append([]byte(nil), value...)
	m.mu.Lock()
	m.data[key] = cp
	m.mu.Unlock()
}

func (m *Mem) Get(key string) ([]byte, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.data[key]
	if !ok {
		return nil, false
	}
	return append([]byte(nil), v...), true
}

func (m *Mem) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.data)
}
