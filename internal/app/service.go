// Package app contains the use cases of the expenses application. It talks
// to the outside world only through the ports declared in the domain.
package app

import (
	"context"
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

	// bgCtx bounds background jobs; Stop cancels it.
	bgCtx    context.Context
	bgCancel context.CancelFunc
	sync     receiptSync
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
		bgCtx:         bgCtx,
		bgCancel:      bgCancel,
	}, nil
}

// resumeDelay gives the HTTP server time to come up before the resumed
// synchronization starts hitting the archive.
const resumeDelay = 10 * time.Second

// Start implements platform.Component. A synchronization interrupted by a
// restart is resumed automatically, so the job needs no page and no click
// to finish loading receipts.
func (s *Service) Start(context.Context) error {
	go s.resumeReceiptSync()
	return nil
}

func (s *Service) resumeReceiptSync() {
	select {
	case <-time.After(resumeDelay):
	case <-s.bgCtx.Done():
		return
	}
	pending, err := s.receipts.PendingDetails(s.bgCtx, 1)
	if err != nil || len(pending) == 0 {
		return
	}
	if _, err := s.StartReceiptSync(s.bgCtx); err != nil {
		s.logger.WarnContext(s.bgCtx, "resume receipt sync", "error", err)
		return
	}
	s.logger.InfoContext(s.bgCtx, "receipt sync resumed after restart")
}

// Stop cancels the background receipt synchronization and waits for it.
func (s *Service) Stop(ctx context.Context) error {
	s.bgCancel()
	s.sync.mu.Lock()
	cancel, done := s.sync.cancel, s.sync.done
	s.sync.mu.Unlock()
	if cancel != nil {
		cancel()
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
