package riverjobs

import (
	"context"
	"testing"
	"time"

	"github.com/riverqueue/river/rivertype"

	"github.com/ALexfonSchneider/goplatform/pkg/postgres"

	"github.com/ALexfonSchneider/expenses/internal/domain"
)

type nopRunner struct{}

func (nopRunner) RunReceiptSync(context.Context) error { return nil }

func TestReceiptSyncArgs_UniqueWhileActive(t *testing.T) {
	opts := ReceiptSyncArgs{}.InsertOpts()
	if opts.Queue != queueReceipts {
		t.Errorf("queue = %q, want %q", opts.Queue, queueReceipts)
	}
	want := map[rivertype.JobState]bool{
		rivertype.JobStateAvailable: true, rivertype.JobStatePending: true, rivertype.JobStateRetryable: true,
		rivertype.JobStateRunning: true, rivertype.JobStateScheduled: true,
	}
	for _, st := range opts.UniqueOpts.ByState {
		delete(want, st)
	}
	if len(want) != 0 {
		t.Errorf("unique states missing: %v", want)
	}
	for _, st := range opts.UniqueOpts.ByState {
		if st == rivertype.JobStateCompleted || st == rivertype.JobStateCancelled || st == rivertype.JobStateDiscarded {
			t.Errorf("a finished job must not block a new run, got %s in unique states", st)
		}
	}
}

func TestNew_Validation(t *testing.T) {
	if _, err := New(nil, nopRunner{}, Config{}); err == nil {
		t.Errorf("nil db must be rejected")
	}
	db, err := postgres.New(postgres.WithDSN("postgres://x:y@localhost:1/z"))
	if err != nil {
		t.Fatalf("postgres.New: %v", err)
	}
	if _, err := New(db, nil, Config{}); err == nil {
		t.Errorf("nil runner must be rejected")
	}
	jobs, err := New(db, nopRunner{}, Config{Interval: time.Hour})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if jobs.Worker() == nil {
		t.Errorf("worker must be built")
	}
	var _ domain.ReceiptSyncScheduler = jobs
	// Before the first tick there is no schedule to derive from.
	next, err := jobs.NextRunAt(context.Background())
	if err != nil || !next.IsZero() {
		t.Errorf("NextRunAt before first tick = %v, %v; want zero, nil", next, err)
	}
	at := time.Date(2026, 9, 12, 19, 0, 0, 0, time.UTC)
	jobs.now = func() time.Time { return at }
	jobs.markTick()
	next, err = jobs.NextRunAt(context.Background())
	if err != nil || !next.Equal(at.Add(time.Hour)) {
		t.Errorf("NextRunAt after tick = %v, %v; want %v", next, err, at.Add(time.Hour))
	}
}
