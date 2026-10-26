package storage

import (
	"context"
	"sync"
)

type memoryObject struct {
	contentType string
	data        []byte
}

type MemoryStore struct {
	mu      sync.RWMutex
	objects map[string]memoryObject
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{objects: make(map[string]memoryObject)}
}

func (m *MemoryStore) Put(_ context.Context, key string, contentType string, data []byte) error {
	copied := append([]byte(nil), data...)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects[key] = memoryObject{contentType: contentType, data: copied}
	return nil
}

func (m *MemoryStore) Get(_ context.Context, key string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	object, ok := m.objects[key]
	if !ok {
		return nil, ErrNotFound
	}
	return append([]byte(nil), object.data...), nil
}

func (m *MemoryStore) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.objects, key)
	return nil
}
