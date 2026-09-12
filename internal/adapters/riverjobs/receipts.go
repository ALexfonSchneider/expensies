// Package riverjobs runs the application's background work on River (a
// Postgres-backed job queue) through the goplatform task engine wrapper.
//
// The receipt synchronization is one job kind. River gives it what a
// goroutine did not: persistence across restarts, a single running
// instance enforced by unique keys, retries with backoff, cancellation of
// jobs that cannot succeed, and a periodic trigger that survives the
// process.
package riverjobs

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/ALexfonSchneider/goplatform/pkg/platform"
	"github.com/ALexfonSchneider/goplatform/pkg/postgres"
	taskriver "github.com/ALexfonSchneider/goplatform/pkg/taskengine/river"

	"github.com/ALexfonSchneider/expenses/internal/domain"
)

// ReceiptSyncArgs is the payload of a receipt synchronization job. It
// carries no data: the job always pulls everything newer than what the
// database holds.
type ReceiptSyncArgs struct {
	// Reason is kept for the job row so the queue is readable by hand.
	Reason string `json:"reason"`
}

// Kind implements river.JobArgs.
func (ReceiptSyncArgs) Kind() string {
	return "receipts.sync"
}

// InsertOpts implements river.JobArgsWithInsertOpts: only one
// synchronization may be queued or running at any time, whoever asks for
// it. A completed job does not block a new one.
func (ReceiptSyncArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue:       queueReceipts,
		MaxAttempts: 3,
		UniqueOpts: river.UniqueOpts{
			ByState: []rivertype.JobState{
				rivertype.JobStateAvailable,
				rivertype.JobStatePending,
				rivertype.JobStateRetryable,
				rivertype.JobStateRunning,
				rivertype.JobStateScheduled,
			},
		},
	}
}

const (
	queueReceipts    = "receipts"
	periodicJobID    = "receipts.sync.periodic"
	syncQueueWorkers = 1
)

// SyncRunner is what the job calls; the application service implements it.
type SyncRunner interface {
	RunReceiptSync(ctx context.Context) error
}

// Config tunes the receipts job.
type Config struct {
	// Interval is the periodic trigger; zero disables it and leaves only
	// manual runs.
	Interval time.Duration
	Logger   platform.Logger
}

// Jobs wraps the task engine worker for this application.
type Jobs struct {
	worker   *taskriver.Worker
	interval time.Duration
	logger   platform.Logger
	now      func() time.Time

	// lastTick is when the periodic schedule last fired (or the worker
	// started), guarded by mu; the next run is one interval later.
	mu       sync.Mutex
	lastTick time.Time
}

// New builds the worker with the receipts queue and its periodic job and
// registers the handler. The worker is a platform.Component: register it
// after postgres and before the HTTP server.
func New(db *postgres.DB, runner SyncRunner, cfg Config) (*Jobs, error) {
	if db == nil {
		return nil, fmt.Errorf("riverjobs: db is required")
	}
	if runner == nil {
		return nil, fmt.Errorf("riverjobs: runner is required")
	}
	if cfg.Logger == nil {
		cfg.Logger = platform.NopLogger()
	}

	opts := []taskriver.WorkerOption{
		taskriver.WithWorkerLogger(cfg.Logger),
		// One worker: the archive throttles clients and two runs would only
		// fight over the same rate limit. The unique key already prevents a
		// second job, this keeps the queue honest about it too.
		taskriver.WithWorkerQueue(queueReceipts, river.QueueConfig{MaxWorkers: syncQueueWorkers}),
	}
	j := &Jobs{interval: cfg.Interval, logger: cfg.Logger, now: time.Now}
	if cfg.Interval > 0 {
		opts = append(opts, taskriver.WithPeriodicJob(river.NewPeriodicJob(
			river.PeriodicInterval(cfg.Interval),
			func() (river.JobArgs, *river.InsertOpts) {
				// River calls the constructor on every tick, including the
				// RunOnStart one, which makes it the clock for NextRunAt.
				j.markTick()
				return ReceiptSyncArgs{Reason: "scheduled"}, nil
			},
			// RunOnStart resumes an interrupted run after a restart and picks
			// up receipts that arrived while the process was down.
			&river.PeriodicJobOpts{ID: periodicJobID, RunOnStart: true},
		)))
	}

	w, err := taskriver.NewWorker(db, opts...)
	if err != nil {
		return nil, fmt.Errorf("riverjobs: new worker: %w", err)
	}
	err = taskriver.RegisterWorker[ReceiptSyncArgs](w, func(ctx context.Context, job *river.Job[ReceiptSyncArgs]) error {
		return runner.RunReceiptSync(ctx)
	})
	if err != nil {
		return nil, fmt.Errorf("riverjobs: register receipts worker: %w", err)
	}
	j.worker = w
	return j, nil
}

func (j *Jobs) markTick() {
	j.mu.Lock()
	j.lastTick = j.now()
	j.mu.Unlock()
}

// Worker returns the platform component to register with the App.
func (j *Jobs) Worker() *taskriver.Worker {
	return j.worker
}

// Enqueue implements domain.ReceiptSyncScheduler.
func (j *Jobs) Enqueue(ctx context.Context) error {
	res, err := j.worker.Insert(ctx, ReceiptSyncArgs{Reason: "manual"}, nil)
	if err != nil {
		return fmt.Errorf("riverjobs: enqueue receipts sync: %w", err)
	}
	if res.UniqueSkippedAsDuplicate {
		return fmt.Errorf("riverjobs: receipts sync is already queued or running: %w", domain.ErrAlreadyExists)
	}
	return nil
}

// NextRunAt implements domain.ReceiptSyncScheduler. River inserts the
// next periodic job only when its timer fires, and while a run is active
// the unique key would swallow that insert anyway, so the queue cannot be
// asked; the answer is derived from the schedule instead.
func (j *Jobs) NextRunAt(context.Context) (time.Time, error) {
	if j.interval <= 0 {
		return time.Time{}, nil
	}
	j.mu.Lock()
	last := j.lastTick
	j.mu.Unlock()
	if last.IsZero() {
		return time.Time{}, nil
	}
	return last.Add(j.interval), nil
}
