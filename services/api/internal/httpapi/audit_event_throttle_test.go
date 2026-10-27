package httpapi

import (
	"fmt"
	"testing"
	"time"
)

func TestAuditEventThrottleAllowsOncePerWindow(t *testing.T) {
	throttle := NewAuditEventThrottle()
	now := time.Date(2026, 10, 27, 8, 0, 0, 0, time.UTC)
	if !throttle.Allow("login:ip-a", now) {
		t.Fatal("first event should be allowed")
	}
	if throttle.Allow("login:ip-a", now.Add(time.Minute)) {
		t.Fatal("duplicate event inside the throttle window should be suppressed")
	}
	if !throttle.Allow("login:ip-a", now.Add(auditEventThrottleWindow)) {
		t.Fatal("event should be allowed after the throttle window")
	}
}

func TestAuditEventThrottleCapsAndSweepsEntries(t *testing.T) {
	throttle := NewAuditEventThrottle()
	now := time.Date(2026, 10, 27, 8, 0, 0, 0, time.UTC)
	for index := 0; index < auditEventThrottleMaxEntries; index++ {
		if !throttle.Allow(fmt.Sprintf("event:%d", index), now) {
			t.Fatalf("event %d unexpectedly rejected", index)
		}
	}
	if throttle.Allow("overflow", now) {
		t.Fatal("event at capacity should be suppressed")
	}
	if got := len(throttle.entries); got != auditEventThrottleMaxEntries {
		t.Fatalf("stored throttle entries = %d, want %d", got, auditEventThrottleMaxEntries)
	}
	if !throttle.Allow("fresh", now.Add(auditEventThrottleWindow+time.Minute)) {
		t.Fatal("new event should be allowed after expired entries are swept")
	}
	if got := len(throttle.entries); got != 1 {
		t.Fatalf("stored throttle entries after sweep = %d, want 1", got)
	}
}
