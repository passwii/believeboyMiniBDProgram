package httpapi

import (
	"sync"
	"time"
)

const (
	adminLoginFailureLimit = 5
	adminLoginWindow       = 10 * time.Minute
	adminLoginCooldown     = 15 * time.Minute
)

type adminLoginState struct {
	failures    []time.Time
	blockedTill time.Time
}

type AdminLoginLimiter struct {
	mu     sync.Mutex
	states map[string]adminLoginState
}

func NewAdminLoginLimiter() *AdminLoginLimiter {
	return &AdminLoginLimiter{states: make(map[string]adminLoginState)}
}

func (l *AdminLoginLimiter) Check(key string, now time.Time) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	state, ok := l.states[key]
	if !ok {
		return 0
	}
	if state.blockedTill.After(now) {
		return state.blockedTill.Sub(now)
	}
	state.blockedTill = time.Time{}
	state.failures = recentFailures(state.failures, now)
	if len(state.failures) == 0 {
		delete(l.states, key)
	} else {
		l.states[key] = state
	}
	return 0
}

func (l *AdminLoginLimiter) RecordFailure(key string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	state := l.states[key]
	state.failures = append(recentFailures(state.failures, now), now)
	if len(state.failures) >= adminLoginFailureLimit {
		state.blockedTill = now.Add(adminLoginCooldown)
		state.failures = nil
	}
	l.states[key] = state
}

func (l *AdminLoginLimiter) RecordSuccess(key string) {
	l.mu.Lock()
	delete(l.states, key)
	l.mu.Unlock()
}

func recentFailures(failures []time.Time, now time.Time) []time.Time {
	cutoff := now.Add(-adminLoginWindow)
	kept := failures[:0]
	for _, failure := range failures {
		if failure.After(cutoff) {
			kept = append(kept, failure)
		}
	}
	return kept
}
