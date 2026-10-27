package httpapi

import (
	"sync"
	"time"
)

const (
	adminLoginFailureLimit  = 5
	adminLoginWindow        = 10 * time.Minute
	adminLoginCooldown      = 15 * time.Minute
	adminLoginBusyRetry     = time.Second
	adminLoginCapacityRetry = time.Minute
	adminLoginMaxStates     = 4096
	adminLoginSweepInterval = time.Minute
)

type adminLoginState struct {
	failures    []time.Time
	blockedTill time.Time
	inFlight    bool
}

type AdminLoginLimiter struct {
	mu        sync.Mutex
	states    map[string]adminLoginState
	lastSweep time.Time
}

type adminLoginAttempt struct {
	limiter *AdminLoginLimiter
	key     string
	once    sync.Once
}

func NewAdminLoginLimiter() *AdminLoginLimiter {
	return &AdminLoginLimiter{states: make(map[string]adminLoginState)}
}

func (l *AdminLoginLimiter) Begin(key string, now time.Time) (*adminLoginAttempt, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.lastSweep.IsZero() || now.Sub(l.lastSweep) >= adminLoginSweepInterval {
		l.sweepLocked(now)
		l.lastSweep = now
	}

	state, exists := l.states[key]
	if exists {
		if state.blockedTill.After(now) {
			return nil, state.blockedTill.Sub(now)
		}
		state.blockedTill = time.Time{}
		state.failures = recentFailures(state.failures, now)
		if state.inFlight {
			return nil, adminLoginBusyRetry
		}
		if len(state.failures) == 0 {
			delete(l.states, key)
			exists = false
		} else {
			l.states[key] = state
		}
	}
	if !exists {
		if len(l.states) >= adminLoginMaxStates {
			return nil, adminLoginCapacityRetry
		}
		state = adminLoginState{}
	}
	state.inFlight = true
	l.states[key] = state
	return &adminLoginAttempt{limiter: l, key: key}, 0
}

func (a *adminLoginAttempt) Failed(now time.Time) {
	a.finish(func(state *adminLoginState) {
		state.failures = append(recentFailures(state.failures, now), now)
		if len(state.failures) >= adminLoginFailureLimit {
			state.blockedTill = now.Add(adminLoginCooldown)
			state.failures = nil
		}
	})
}

func (a *adminLoginAttempt) Succeeded() {
	a.finish(nil)
}

func (a *adminLoginAttempt) Release(now time.Time) {
	a.finish(func(state *adminLoginState) {
		state.failures = recentFailures(state.failures, now)
	})
}

func (a *adminLoginAttempt) finish(update func(*adminLoginState)) {
	a.once.Do(func() {
		l := a.limiter
		l.mu.Lock()
		defer l.mu.Unlock()
		state, exists := l.states[a.key]
		if !exists || !state.inFlight {
			return
		}
		state.inFlight = false
		if update == nil {
			delete(l.states, a.key)
			return
		}
		update(&state)
		if state.blockedTill.IsZero() && len(state.failures) == 0 {
			delete(l.states, a.key)
			return
		}
		l.states[a.key] = state
	})
}

func (l *AdminLoginLimiter) sweepLocked(now time.Time) {
	for key, state := range l.states {
		if state.inFlight || state.blockedTill.After(now) {
			continue
		}
		state.blockedTill = time.Time{}
		state.failures = recentFailures(state.failures, now)
		if len(state.failures) == 0 {
			delete(l.states, key)
		} else {
			l.states[key] = state
		}
	}
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
