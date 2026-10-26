package share

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	ipFailureLimit      = 5
	ipWindow            = 10 * time.Minute
	ipBlockDuration     = 15 * time.Minute
	globalFailureLimit  = 100
	globalWindow        = 10 * time.Minute
	globalBlockDuration = 5 * time.Minute
)

type LimiterDecision struct {
	Allowed    bool
	RetryAfter time.Duration
}

type failureWindow struct {
	count        int
	windowStart  time.Time
	blockedUntil time.Time
}

type Limiter struct {
	mu      sync.Mutex
	ips     map[string]*failureWindow
	globals map[uuid.UUID]*failureWindow
}

func NewLimiter() *Limiter {
	return &Limiter{
		ips: make(map[string]*failureWindow), globals: make(map[uuid.UUID]*failureWindow),
	}
}

func (l *Limiter) Check(shareID uuid.UUID, ip string, now time.Time) LimiterDecision {
	l.mu.Lock()
	defer l.mu.Unlock()
	if blocked, retry := blockedFor(l.globals[shareID], now); blocked {
		return LimiterDecision{RetryAfter: retry}
	}
	if blocked, retry := blockedFor(l.ips[ipKey(shareID, ip)], now); blocked {
		return LimiterDecision{RetryAfter: retry}
	}
	return LimiterDecision{Allowed: true}
}

func (l *Limiter) RecordFailure(shareID uuid.UUID, ip string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	key := ipKey(shareID, ip)
	l.ips[key] = addFailure(l.ips[key], now, ipFailureLimit, ipBlockDuration)
	l.globals[shareID] = addFailure(l.globals[shareID], now, globalFailureLimit, globalBlockDuration)
}

func (l *Limiter) RecordSuccess(shareID uuid.UUID, ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.ips, ipKey(shareID, ip))
}

func addFailure(window *failureWindow, now time.Time, limit int, block time.Duration) *failureWindow {
	if window == nil || now.Sub(window.windowStart) >= globalWindow {
		window = &failureWindow{windowStart: now}
	}
	window.count++
	if window.count >= limit {
		window.blockedUntil = now.Add(block)
		window.count = 0
		window.windowStart = now
	}
	return window
}

func blockedFor(window *failureWindow, now time.Time) (bool, time.Duration) {
	if window == nil || !window.blockedUntil.After(now) {
		return false, 0
	}
	return true, window.blockedUntil.Sub(now)
}

func ipKey(shareID uuid.UUID, ip string) string {
	return shareID.String() + ":" + ip
}
