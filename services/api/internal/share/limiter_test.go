package share

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestLimiterBlocksAfterFiveIPFailures(t *testing.T) {
	limiter := NewLimiter()
	shareID := uuid.New()
	now := time.Date(2026, 10, 27, 1, 0, 0, 0, time.UTC)

	if decision := limiter.Check(shareID, "ip-a", now); !decision.Allowed {
		t.Fatal("initial request should be allowed")
	}
	for i := 0; i < 5; i++ {
		limiter.RecordFailure(shareID, "ip-a", now)
	}
	decision := limiter.Check(shareID, "ip-a", now.Add(time.Second))
	if decision.Allowed || decision.RetryAfter <= 0 {
		t.Fatalf("decision = %+v", decision)
	}
	if decision := limiter.Check(shareID, "ip-a", now.Add(16*time.Minute)); !decision.Allowed {
		t.Fatal("request should be allowed after cooldown")
	}
}

func TestLimiterGlobalCooldownAndSuccessReset(t *testing.T) {
	limiter := NewLimiter()
	shareID := uuid.New()
	now := time.Date(2026, 10, 27, 1, 0, 0, 0, time.UTC)

	limiter.RecordFailure(shareID, "ip-a", now)
	limiter.RecordSuccess(shareID, "ip-a")
	for i := 0; i < 99; i++ {
		limiter.RecordFailure(shareID, "ip-b", now)
	}
	limiter.RecordFailure(shareID, "ip-b", now)
	decision := limiter.Check(shareID, "ip-c", now)
	if decision.Allowed || decision.RetryAfter <= 0 {
		t.Fatalf("global decision = %+v", decision)
	}
}
