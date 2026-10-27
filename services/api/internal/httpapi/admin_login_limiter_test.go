package httpapi

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestAdminLoginLimiterBlocksAfterFiveFailures(t *testing.T) {
	limiter := NewAdminLoginLimiter()
	now := time.Date(2026, 10, 27, 8, 0, 0, 0, time.UTC)
	key := "hmac-ip-a"

	for attempt := 1; attempt <= 5; attempt++ {
		permit, retry := limiter.Begin(key, now)
		if retry > 0 || permit == nil {
			t.Fatalf("attempt %d blocked for %v", attempt, retry)
		}
		permit.Failed(now.Add(time.Duration(attempt) * time.Second))
	}
	if permit, retry := limiter.Begin(key, now.Add(6*time.Second)); permit != nil || retry <= 0 {
		t.Fatal("expected sixth attempt to be blocked")
	}
	if permit, retry := limiter.Begin(key, now.Add(16*time.Minute)); permit == nil || retry > 0 {
		t.Fatalf("cooldown should expire, remaining=%v", retry)
	} else {
		permit.Succeeded()
	}
}

func TestAdminLoginLimiterUsesSlidingWindowAndSuccessClears(t *testing.T) {
	limiter := NewAdminLoginLimiter()
	base := time.Date(2026, 10, 27, 8, 0, 0, 0, time.UTC)
	key := "hmac-ip-b"
	for attempt := 0; attempt < 4; attempt++ {
		permit, retry := limiter.Begin(key, base.Add(time.Duration(attempt)*time.Minute))
		if permit == nil || retry > 0 {
			t.Fatalf("attempt %d unexpectedly blocked for %v", attempt, retry)
		}
		permit.Failed(base.Add(time.Duration(attempt) * time.Minute))
	}
	permit, retry := limiter.Begin(key, base.Add(11*time.Minute))
	if permit == nil || retry > 0 {
		t.Fatalf("old failures should slide out, remaining=%v", retry)
	}
	permit.Failed(base.Add(12 * time.Minute))
	permit, retry = limiter.Begin(key, base.Add(13*time.Minute))
	if permit == nil || retry > 0 {
		t.Fatalf("successful login should reset failures, remaining=%v", retry)
	}
	permit.Succeeded()
}

func TestAdminLoginLimiterRejectsConcurrentAttemptsAndBlocksAfterFailures(t *testing.T) {
	limiter := NewAdminLoginLimiter()
	now := time.Now()
	first, retry := limiter.Begin("same-ip", now)
	if first == nil || retry > 0 {
		t.Fatalf("first attempt blocked for %v", retry)
	}
	var group sync.WaitGroup
	for attempt := 0; attempt < 50; attempt++ {
		group.Add(1)
		go func() {
			defer group.Done()
			if permit, retry := limiter.Begin("same-ip", now); permit != nil || retry <= 0 {
				t.Errorf("concurrent attempt = (%v, %v), want denied", permit, retry)
				if permit != nil {
					permit.Release(now)
				}
			}
		}()
	}
	group.Wait()
	first.Failed(now)
	for attempt := 1; attempt < adminLoginFailureLimit; attempt++ {
		permit, retry := limiter.Begin("same-ip", now.Add(time.Duration(attempt)*time.Second))
		if permit == nil || retry > 0 {
			t.Fatalf("attempt %d unexpectedly blocked for %v", attempt+1, retry)
		}
		permit.Failed(now.Add(time.Duration(attempt) * time.Second))
	}
	if permit, retry := limiter.Begin("same-ip", now.Add(6*time.Second)); permit != nil || retry <= 0 {
		t.Fatal("expected fifth failure to block subsequent attempts")
	}
}

func TestAdminLoginLimiterBoundsStoredIPState(t *testing.T) {
	limiter := NewAdminLoginLimiter()
	now := time.Now()
	for ip := 0; ip < 5000; ip++ {
		permit, _ := limiter.Begin(fmt.Sprintf("ip-%d", ip), now)
		if permit != nil {
			permit.Failed(now)
		}
	}
	if len(limiter.states) > adminLoginMaxStates {
		t.Fatalf("stored IP states = %d, want no more than %d", len(limiter.states), adminLoginMaxStates)
	}
	if permit, retry := limiter.Begin("overflow-ip", now); permit != nil || retry <= 0 {
		t.Fatalf("new IP at capacity = (%v, %v), want fail-closed retry", permit, retry)
	}
	permit, retry := limiter.Begin("new-ip", now.Add(11*time.Minute))
	if permit == nil || retry > 0 {
		t.Fatalf("expired state should be swept to admit a new IP, retry=%v", retry)
	}
	permit.Release(now.Add(11 * time.Minute))
	if len(limiter.states) != 0 {
		t.Fatalf("stored IP states after expiry sweep = %d, want 0", len(limiter.states))
	}
}
