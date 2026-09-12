// Package app contains the use cases of the expenses application. It talks
// to the outside world only through the ports declared in the domain.
package app

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ALexfonSchneider/goplatform/pkg/platform"

	"github.com/ALexfonSchneider/expenses/internal/domain"
)

// Deps lists the collaborators of Service.
type Deps struct {
	Logger          platform.Logger
	Parser          domain.StatementParser
	Statements      domain.StatementRepository
	Transactions    domain.TransactionRepository
	Categories      domain.CategoryRepository
	Analytics       domain.AnalyticsRepository
	Budgets         domain.BudgetRepository
	ReceiptSource   domain.ReceiptSource
	ReceiptSessions domain.ReceiptSessionStore
	ReceiptSync     domain.ReceiptSyncStore
	Receipts        domain.ReceiptRepository
	// ReceiptSyncInterval is how often receipts are pulled from the archive
	// without anyone pressing the button; zero leaves only the manual run.
	ReceiptSyncInterval time.Duration
	// Now supplies the current time once per operation so repositories
	// never read the clock themselves.
	Now func() time.Time
}

// Service implements the use cases. It is also a platform.Component so the
// application can stop its background receipt synchronization in order.
type Service struct {
	logger        platform.Logger
	parser        domain.StatementParser
	statements    domain.StatementRepository
	transactions  domain.TransactionRepository
	categories    domain.CategoryRepository
	analytics     domain.AnalyticsRepository
	budgets       domain.BudgetRepository
	receiptSource domain.ReceiptSource
	sessions      domain.ReceiptSessionStore
	syncStore     domain.ReceiptSyncStore
	receipts      domain.ReceiptRepository
	now           func() time.Time
	syncInterval  time.Duration

	// bgCtx bounds background jobs; Stop cancels it.
	bgCtx    context.Context
	bgCancel context.CancelFunc
	sync     receiptSync
	// schedulerDone closes when the periodic sync loop has exited.
	schedulerDone chan struct{}
}

// receiptSync is the state of the single background synchronization.
type receiptSync struct {
	mu     sync.Mutex
	status domain.ReceiptSyncStatus
	loaded bool
	cancel context.CancelFunc
	done   chan struct{}
}

// NewService validates the dependencies and builds a Service.
func NewService(d Deps) (*Service, error) {
	switch {
	case d.Parser == nil:
		return nil, fmt.Errorf("app: parser is required")
	case d.Statements == nil:
		return nil, fmt.Errorf("app: statement repository is required")
	case d.Transactions == nil:
		return nil, fmt.Errorf("app: transaction repository is required")
	case d.Categories == nil:
		return nil, fmt.Errorf("app: category repository is required")
	case d.Analytics == nil:
		return nil, fmt.Errorf("app: analytics repository is required")
	case d.Budgets == nil:
		return nil, fmt.Errorf("app: budget repository is required")
	case d.ReceiptSource == nil:
		return nil, fmt.Errorf("app: receipt source is required")
	case d.ReceiptSessions == nil:
		return nil, fmt.Errorf("app: receipt session store is required")
	case d.ReceiptSync == nil:
		return nil, fmt.Errorf("app: receipt sync store is required")
	case d.Receipts == nil:
		return nil, fmt.Errorf("app: receipt repository is required")
	}
	if d.Logger == nil {
		d.Logger = platform.NopLogger()
	}
	if d.Now == nil {
		d.Now = time.Now
	}
	bgCtx, bgCancel := context.WithCancel(context.Background())
	return &Service{
		logger:        d.Logger,
		parser:        d.Parser,
		statements:    d.Statements,
		transactions:  d.Transactions,
		categories:    d.Categories,
		analytics:     d.Analytics,
		budgets:       d.Budgets,
		receiptSource: d.ReceiptSource,
		sessions:      d.ReceiptSessions,
		syncStore:     d.ReceiptSync,
		receipts:      d.Receipts,
		now:           d.Now,
		syncInterval:  d.ReceiptSyncInterval,
		bgCtx:         bgCtx,
		bgCancel:      bgCancel,
		schedulerDone: make(chan struct{}),
	}, nil
}

// startupDelay gives the HTTP server time to come up before background
// work starts hitting the archive.
const startupDelay = 10 * time.Second

// Start implements platform.Component. It launches the receipt scheduler:
// an interrupted synchronization is resumed right away, and new receipts
// are pulled every ReceiptSyncInterval, so nothing depends on a page being
// open or a button being pressed.
func (s *Service) Start(context.Context) error {
	go s.runReceiptScheduler()
	return nil
}

func (s *Service) runReceiptScheduler() {
	defer close(s.schedulerDone)
	select {
	case <-time.After(startupDelay):
	case <-s.bgCtx.Done():
		return
	}
	s.syncIfDue(true)
	if s.syncInterval <= 0 {
		return
	}
	ticker := time.NewTicker(s.syncInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.syncIfDue(false)
		case <-s.bgCtx.Done():
			return
		}
	}
}

// syncIfDue starts a synchronization when a session is configured and
// either a previous run was interrupted (receipts without details remain),
// the last run is older than the interval, or a tick asks for it. Without
// a session it stays silent: the overview already shows that signal.
func (s *Service) syncIfDue(startup bool) {
	ctx := s.bgCtx
	session, err := s.sessions.Load(ctx)
	if err != nil || session == nil {
		return
	}
	reason := "scheduled"
	if startup {
		pending, err := s.receipts.PendingDetails(ctx, 1)
		if err != nil {
			return
		}
		status, err := s.ReceiptSyncStatus(ctx)
		if err != nil {
			return
		}
		switch {
		case len(pending) > 0:
			reason = "resumed after restart"
		case s.syncInterval > 0 && (status.FinishedAt.IsZero() || s.now().Sub(status.FinishedAt) >= s.syncInterval):
			reason = "overdue at startup"
		default:
			return
		}
	}
	if _, err := s.StartReceiptSync(ctx); err != nil {
		if !errors.Is(err, domain.ErrAlreadyExists) {
			s.logger.WarnContext(ctx, "start receipt sync", "reason", reason, "error", err)
		}
		return
	}
	s.logger.InfoContext(ctx, "receipt sync started", "reason", reason)
}

// Stop cancels the scheduler and the running synchronization and waits
// for both.
func (s *Service) Stop(ctx context.Context) error {
	s.bgCancel()
	s.sync.mu.Lock()
	cancel, done := s.sync.cancel, s.sync.done
	s.sync.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	select {
	case <-s.schedulerDone:
	case <-ctx.Done():
		return fmt.Errorf("app: stop scheduler: %w", ctx.Err())
	}
	if done == nil {
		return nil
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("app: stop: %w", ctx.Err())
	}
}
