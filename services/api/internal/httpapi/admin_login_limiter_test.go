package httpapi

import (
	"sync"
	"testing"
	"time"
)

func TestAdminLoginLimiterBlocksAfterFiveFailures(t *testing.T) {
	limiter := NewAdminLoginLimiter()
	now := time.Date(2026, 10, 27, 8, 0, 0, 0, time.UTC)
	key := "hmac-ip-a"

	for attempt := 1; attempt <= 5; attempt++ {
		if retry := limiter.Check(key, now); retry > 0 {
			t.Fatalf("attempt %d blocked for %v", attempt, retry)
		}
		limiter.RecordFailure(key, now.Add(time.Duration(attempt)*time.Second))
	}
	if retry := limiter.Check(key, now.Add(6*time.Second)); retry <= 0 {
		t.Fatal("expected sixth attempt to be blocked")
	}
	if retry := limiter.Check(key, now.Add(16*time.Minute)); retry > 0 {
		t.Fatalf("cooldown should expire, remaining=%v", retry)
	}
}

func TestAdminLoginLimiterUsesSlidingWindowAndSuccessClears(t *testing.T) {
	limiter := NewAdminLoginLimiter()
	base := time.Date(2026, 10, 27, 8, 0, 0, 0, time.UTC)
	key := "hmac-ip-b"
	for attempt := 0; attempt < 4; attempt++ {
		limiter.RecordFailure(key, base.Add(time.Duration(attempt)*time.Minute))
	}
	if retry := limiter.Check(key, base.Add(11*time.Minute)); retry > 0 {
		t.Fatalf("old failures should slide out, remaining=%v", retry)
	}
	limiter.RecordFailure(key, base.Add(12*time.Minute))
	limiter.RecordSuccess(key)
	if retry := limiter.Check(key, base.Add(13*time.Minute)); retry > 0 {
		t.Fatalf("successful login should reset failures, remaining=%v", retry)
	}
}

func TestAdminLoginLimiterConcurrentFailuresRemainBlocked(t *testing.T) {
	limiter := NewAdminLoginLimiter()
	now := time.Now()
	var group sync.WaitGroup
	for attempt := 0; attempt < 50; attempt++ {
		group.Add(1)
		go func() {
			defer group.Done()
			limiter.RecordFailure("same-ip", now)
		}()
	}
	group.Wait()
	if retry := limiter.Check("same-ip", now); retry <= 0 {
		t.Fatal("expected concurrent failures to block the IP")
	}
}
