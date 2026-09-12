// Package app contains the use cases of the expenses application. It talks
// to the outside world only through the ports declared in the domain.
package app

import (
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

// Service implements the use cases. Background work is not started here:
// the receipt synchronization is a job run by the task engine, which
// calls back into RunReceiptSync.
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
	// scheduler is set after construction because the task engine needs
	// the service (to register the job handler) and the service needs the
	// engine (to enqueue); see SetReceiptSyncScheduler.
	scheduler domain.ReceiptSyncScheduler

	sync receiptSync
}

// receiptSync is the in-memory view of the synchronization that is
// currently running (or the last one, once loaded from the store).
type receiptSync struct {
	mu     sync.Mutex
	status domain.ReceiptSyncStatus
	loaded bool
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
	}, nil
}

// SetReceiptSyncScheduler wires the task engine in once it exists. Until
// then StartReceiptSync reports that scheduling is unavailable.
func (s *Service) SetReceiptSyncScheduler(sched domain.ReceiptSyncScheduler) {
	s.scheduler = sched
}
