package maintenance

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"
)

type fakeStore struct {
	calls  atomic.Int64
	result Counts
	err    error
	called chan time.Time
}

func (f *fakeStore) CleanupExpired(_ context.Context, now time.Time) (Counts, error) {
	f.calls.Add(1)
	if f.called != nil {
		select {
		case f.called <- now:
		default:
		}
	}
	return f.result, f.err
}

func TestRunPerformsInitialCleanupAndStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store := &fakeStore{called: make(chan time.Time, 4)}
	done := make(chan struct{})
	go func() {
		Run(ctx, store, slog.New(slog.NewTextHandler(io.Discard, nil)), time.Hour)
		close(done)
	}()
	select {
	case <-store.called:
	case <-time.After(time.Second):
		t.Fatal("cleanup did not run at startup")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cleanup runner did not stop after cancellation")
	}
}

func TestRunRepeatsOnInterval(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	store := &fakeStore{called: make(chan time.Time, 8)}
	go Run(ctx, store, slog.New(slog.NewTextHandler(io.Discard, nil)), 10*time.Millisecond)
	for run := 0; run < 2; run++ {
		select {
		case <-store.called:
		case <-time.After(time.Second):
			cancel()
			t.Fatalf("cleanup run %d did not happen", run+1)
		}
	}
	cancel()
}

func TestRunKeepsRunningAfterCleanupError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	store := &fakeStore{called: make(chan time.Time, 8), err: errors.New("temporary failure")}
	go Run(ctx, store, slog.New(slog.NewTextHandler(io.Discard, nil)), 10*time.Millisecond)
	for run := 0; run < 2; run++ {
		select {
		case <-store.called:
		case <-time.After(time.Second):
			cancel()
			t.Fatalf("cleanup run %d did not happen after error", run+1)
		}
	}
	cancel()
}

func TestRunLogsDeletedRowCounts(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store := &fakeStore{
		called: make(chan time.Time, 1),
		result: Counts{AdminSessions: 2, AccessSessions: 3, ViewerTickets: 4, AuditEvents: 5},
	}
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	done := make(chan struct{})
	go func() {
		Run(ctx, store, logger, time.Hour)
		close(done)
	}()
	select {
	case <-store.called:
	case <-time.After(time.Second):
		cancel()
		t.Fatal("cleanup did not run")
	}
	cancel()
	<-done
	var event map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &event); err != nil {
		t.Fatalf("decode log record: %v", err)
	}
	if event["admin_sessions"] != float64(2) || event["access_sessions"] != float64(3) ||
		event["viewer_tickets"] != float64(4) || event["audit_events"] != float64(5) {
		t.Fatalf("cleanup log = %v", event)
	}
}
