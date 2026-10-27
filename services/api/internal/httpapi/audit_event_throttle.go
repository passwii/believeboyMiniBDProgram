package httpapi

import (
	"sync"
	"time"
)

const (
	auditEventThrottleWindow     = 15 * time.Minute
	auditEventThrottleSweep      = time.Minute
	auditEventThrottleMaxEntries = 4096
)

type AuditEventThrottle struct {
	mu        sync.Mutex
	entries   map[string]time.Time
	lastSweep time.Time
}

func NewAuditEventThrottle() *AuditEventThrottle {
	return &AuditEventThrottle{entries: make(map[string]time.Time)}
}

func (t *AuditEventThrottle) Allow(key string, now time.Time) bool {
	if t == nil {
		return true
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.lastSweep.IsZero() || now.Sub(t.lastSweep) >= auditEventThrottleSweep {
		for entry, expiresAt := range t.entries {
			if !expiresAt.After(now) {
				delete(t.entries, entry)
			}
		}
		t.lastSweep = now
	}

	if expiresAt, exists := t.entries[key]; exists {
		if expiresAt.After(now) {
			return false
		}
		delete(t.entries, key)
	}
	if len(t.entries) >= auditEventThrottleMaxEntries {
		return false
	}
	t.entries[key] = now.Add(auditEventThrottleWindow)
	return true
}
