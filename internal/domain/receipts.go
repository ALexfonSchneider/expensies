package domain

import (
	"context"
	"errors"
	"time"
)

// Receipt-related errors.
var (
	// ErrReceiptSession means the tax service credentials are missing or
	// no longer accepted; the user has to sign in with a browser again.
	ErrReceiptSession = errors.New("receipt service session is missing or expired")
	// ErrReceiptDetailsUnavailable means the archive knows the receipt but
	// has no fiscal details for it (yet).
	ErrReceiptDetailsUnavailable = errors.New("receipt details are not available")
)

// ReceiptSession is the credential set for the tax service receipt
// archive. The user obtains it from a browser session; the application
// only refreshes it and never performs the interactive login.
type ReceiptSession struct {
	Phone            string
	DeviceID         string
	AccessToken      string
	RefreshToken     string
	TokenExpiresAt   time.Time
	RefreshExpiresAt time.Time
	UpdatedAt        time.Time
}

// ReceiptSessionStore persists the single session.
type ReceiptSessionStore interface {
	// Load returns nil, nil when no session was configured.
	Load(ctx context.Context) (*ReceiptSession, error)
	Save(ctx context.Context, s *ReceiptSession) error
	Clear(ctx context.Context) error
}

// ReceiptSyncStore keeps the outcome of the last synchronization so the UI
// can show it after a restart.
type ReceiptSyncStore interface {
	LoadSyncStatus(ctx context.Context) (*ReceiptSyncStatus, error)
	SaveSyncStatus(ctx context.Context, st *ReceiptSyncStatus, now time.Time) error
}

// ReceiptSource pulls receipts from the tax service archive.
type ReceiptSource interface {
	// ListReceipts returns receipt headers newest first, one page at a time.
	ListReceipts(ctx context.Context, since *time.Time, offset, limit int) ([]Receipt, bool, error)
	// LoadDetails fills seller, timestamps, totals and items of r.
	LoadDetails(ctx context.Context, r *Receipt) error
	// Probe makes the cheapest authenticated call to validate the session.
	Probe(ctx context.Context) error
}

// ReceiptMatchKind says how a receipt got linked to a transaction.
type ReceiptMatchKind string

const (
	ReceiptMatchNone   ReceiptMatchKind = ""
	ReceiptMatchAuto   ReceiptMatchKind = "auto"
	ReceiptMatchManual ReceiptMatchKind = "manual"
	// ReceiptMatchVirtual means the receipt is represented by a
	// transaction of its own because no statement covers it yet.
	ReceiptMatchVirtual ReceiptMatchKind = "virtual"
)

// Unmatched reports whether the receipt still lacks a bank line; a
// virtual transaction does not count as a match.
func (r Receipt) Unmatched() bool {
	return r.TransactionID == nil || r.MatchKind == ReceiptMatchVirtual
}

// IsPayment reports whether money actually moved with this document. A
// delivery prints a second receipt on hand-over that only offsets the
// prepayment, and a purchase on credit prints one with no payment at all;
// both carry zero cash and zero card totals.
func (r Receipt) IsPayment() bool {
	return r.ItemsLoaded && r.CashTotal+r.EcashTotal > 0
}

// Fiscal operation types as printed on receipts.
const (
	ReceiptOpSale       = 1
	ReceiptOpSaleRefund = 2
	ReceiptOpBuy        = 3
	ReceiptOpBuyRefund  = 4
)

// Receipt is one fiscal document from the archive.
type Receipt struct {
	ID                   int64
	Source               string
	Key                  string
	FiscalDriveNumber    string
	FiscalDocumentNumber string
	FiscalSign           string
	SellerName           string
	SellerINN            string
	RetailPlace          string
	RetailAddress        string
	IssuedAt             time.Time
	ReceivedAt           time.Time
	OperationType        int
	Total                Money
	CashTotal            Money
	EcashTotal           Money
	ItemsLoaded          bool
	ItemsError           string
	TransactionID        *int64
	MatchKind            ReceiptMatchKind
	ItemCount            int
	Items                []ReceiptItem
}

// Direction derives the money direction from the fiscal operation type.
func (r Receipt) Direction() Direction {
	switch r.OperationType {
	case ReceiptOpSaleRefund, ReceiptOpBuy:
		return DirectionIncome
	default:
		return DirectionExpense
	}
}

// ReceiptItem is one line of a receipt.
type ReceiptItem struct {
	ID          int64
	ReceiptID   int64
	Position    int
	Name        string
	Price       Money
	Quantity    float64
	Sum         Money
	ProductType int
	CategoryID  *int64
}

// ReceiptFilter selects receipts for listing by the fiscal date.
type ReceiptFilter struct {
	From      *time.Time
	To        *time.Time
	Unmatched bool
	Limit     int
	Offset    int
}

// ReceiptSyncStatus reports one synchronization run.
type ReceiptSyncStatus struct {
	Running    bool
	StartedAt  time.Time
	FinishedAt time.Time
	Listed     int
	Added      int
	Detailed   int
	Pending    int
	Matched    int
	Error      string
	// NextRunAt is when the scheduler will pull receipts again; zero when
	// the scheduler is off or nothing is connected.
	NextRunAt time.Time
}

// ItemTotal aggregates one product name over a period.
type ItemTotal struct {
	Name     string
	Quantity float64
	Sum      Money
	Count    int
}

// ReceiptRepository persists receipts and their links to transactions.
type ReceiptRepository interface {
	// UpsertHeader stores a listing entry, keeping details of an existing
	// row. It fills r.ID and r.ItemsLoaded and reports whether the row is new.
	UpsertHeader(ctx context.Context, r *Receipt, now time.Time) (bool, error)
	// SaveDetails replaces seller data, totals and items in one transaction.
	SaveDetails(ctx context.Context, r *Receipt) error
	SetItemsError(ctx context.Context, id int64, msg string) error
	PendingDetails(ctx context.Context, limit int) ([]Receipt, error)
	List(ctx context.Context, f ReceiptFilter) ([]Receipt, int, error)
	Get(ctx context.Context, id int64) (*Receipt, error)
	// ByTransaction returns every receipt of a transaction with items,
	// oldest first; a delivery often fiscalizes one payment as several.
	ByTransaction(ctx context.Context, txID int64) ([]Receipt, error)
	// Unmatched lists receipts without a bank line, including those that
	// currently stand in as virtual transactions.
	Unmatched(ctx context.Context) ([]Receipt, error)
	// Candidates lists statement transactions of the amount and direction
	// whose time falls in [from, to] and that own no receipt yet.
	Candidates(ctx context.Context, total Money, dir Direction, from, to time.Time) ([]Transaction, error)
	// Link attaches a receipt to a statement transaction. An automatic
	// link never overrides a bank link and reports ErrAlreadyExists
	// instead; a manual link replaces whatever was there.
	Link(ctx context.Context, id, txID int64, kind ReceiptMatchKind) error
	// AttachVirtual stores tx as a receipt-born transaction and links the
	// receipt to it in one step. The receipt must be unmatched.
	AttachVirtual(ctx context.Context, receiptID int64, tx *Transaction, now time.Time) error
	// DropVirtual removes the virtual transaction of a receipt and returns
	// it (nil when the receipt had none) so its edits can be carried over.
	DropVirtual(ctx context.Context, receiptID int64) (*Transaction, error)
	Unlink(ctx context.Context, id int64) error
	LatestReceivedAt(ctx context.Context) (time.Time, bool, error)
	TopItems(ctx context.Context, q RangeQuery, limit int) ([]ItemTotal, error)
}
