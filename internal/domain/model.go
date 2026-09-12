// Package domain holds the entities, value types and ports of the expenses
// application. It depends on nothing outside the standard library so every
// adapter (PDF parser, Postgres, HTTP) points inwards.
package domain

import "time"

// Money is an amount in minor currency units (kopecks). Integer arithmetic
// keeps sums exact, which matters when parsed totals are checked against
// the totals printed on the statement.
type Money int64

// Direction says whether money left the account or arrived on it.
type Direction string

const (
	DirectionExpense Direction = "expense"
	DirectionIncome  Direction = "income"
)

// Kind is the bank-level nature of an operation derived from its
// description. It is fixed at import time; user categories are layered on
// top and can change later.
type Kind string

const (
	KindPurchase    Kind = "purchase"
	KindTransferIn  Kind = "transfer_in"
	KindTransferOut Kind = "transfer_out"
	KindRefund      Kind = "refund"
	KindCashback    Kind = "cashback"
	KindOther       Kind = "other"
)

// CategorySource records who assigned the category, so re-applying rules
// never overwrites a manual decision.
type CategorySource string

const (
	CategorySourceNone   CategorySource = ""
	CategorySourceRule   CategorySource = "rule"
	CategorySourceManual CategorySource = "manual"
)

// TransactionSource says where a transaction came from.
type TransactionSource string

const (
	// SourceStatement is a line of a bank statement.
	SourceStatement TransactionSource = "statement"
	// SourceReceipt is a receipt with no bank line: it stands in for the
	// payment until a statement that contains it is uploaded.
	SourceReceipt TransactionSource = "receipt"
)

// Transaction is one spending line: a bank statement entry or a receipt
// that no statement covers yet.
type Transaction struct {
	ID int64
	// StatementID is zero for receipt-born transactions.
	StatementID int64
	Source      TransactionSource
	// Fingerprint identifies the operation across statements so overlapping
	// periods can be uploaded without creating duplicates.
	Fingerprint string
	OpAt        time.Time
	// ProcessedOn is the bank-side settlement date. Queries and aggregates
	// use OpAt because that is when the money was actually spent.
	ProcessedOn time.Time
	Card        string
	// Amount is always positive; Direction carries the sign.
	Amount         Money
	Direction      Direction
	Kind           Kind
	Description    string
	Merchant       string
	MCC            string
	Counterparty   string
	CategoryID     *int64
	CategorySource CategorySource
	Note           string
	// Excluded drops the line from every aggregate while keeping it in the
	// list, for one-off purchases that would distort averages and trends.
	Excluded bool
	// ReceiptID points at the fiscal receipt matched to this line, if any.
	ReceiptID *int64
}

// Statement is one uploaded PDF.
type Statement struct {
	ID             int64
	Bank           string
	Number         string
	PeriodFrom     time.Time
	PeriodTo       time.Time
	OpeningBalance Money
	ClosingBalance Money
	// TotalIncome and TotalExpense are the totals printed by the bank; the
	// parser compares them with the sum of parsed transactions.
	TotalIncome  Money
	TotalExpense Money
	FileName     string
	FileSHA256   string
	TxCount      int
	UploadedAt   time.Time
}

// ParsedStatement is the output of a StatementParser before persistence.
type ParsedStatement struct {
	Statement    Statement
	Transactions []Transaction
	// Warnings are non-fatal parsing problems (totals mismatch, missing
	// time) that the UI shows after import instead of silently dropping data.
	Warnings []string
}

// Category is a user-defined spending group.
type Category struct {
	ID    int64
	Name  string
	Color string
	// IsTransfer marks money moved between accounts or people. Such
	// categories are excluded from spending analytics unless asked for,
	// because a top-up from the user's own card is not an expense.
	IsTransfer bool
	SortOrder  int
}

// RuleField is the transaction attribute a Rule inspects.
type RuleField string

const (
	RuleFieldMerchant     RuleField = "merchant"
	RuleFieldDescription  RuleField = "description"
	RuleFieldMCC          RuleField = "mcc"
	RuleFieldKind         RuleField = "kind"
	RuleFieldCounterparty RuleField = "counterparty"
)

// RuleOp is how a Rule compares the field with its value.
type RuleOp string

const (
	RuleOpContains RuleOp = "contains"
	RuleOpEquals   RuleOp = "equals"
	RuleOpPrefix   RuleOp = "prefix"
	RuleOpRegex    RuleOp = "regex"
)

// Rule assigns a category to transactions whose field matches. Rules are
// evaluated in ascending Priority order; the first match wins.
type Rule struct {
	ID         int64
	CategoryID int64
	Field      RuleField
	Op         RuleOp
	Value      string
	Priority   int
}

// Granularity selects the bucket size of a time series.
type Granularity string

const (
	GranularityDay   Granularity = "day"
	GranularityWeek  Granularity = "week"
	GranularityMonth Granularity = "month"
)

// RangeQuery bounds analytics to the inclusive calendar dates [From, To].
// Dates are time.Time values at midnight UTC.
type RangeQuery struct {
	From             time.Time
	To               time.Time
	IncludeTransfers bool
}

// SeriesQuery is a RangeQuery with a bucket size.
type SeriesQuery struct {
	RangeQuery
	Granularity Granularity
}

// Bucket is one point of a time series.
type Bucket struct {
	Start   time.Time
	Expense Money
	Income  Money
	Count   int
}

// Totals is the aggregate of a period.
type Totals struct {
	Expense Money
	Income  Money
	Count   int
}

// Summary is the dashboard headline for a period.
type Summary struct {
	From            time.Time
	To              time.Time
	Expense         Money
	Income          Money
	Count           int
	Days            int
	ActiveDays      int
	AvgPerDay       Money
	AvgPerActiveDay Money
	MaxDay          *Bucket
	// Prev is the same-length period ending the day before From, used for
	// trend arrows.
	Prev Totals
}

// CategoryTotal is the aggregate of one category over a period. A nil
// CategoryID stands for uncategorized transactions.
type CategoryTotal struct {
	CategoryID *int64
	Name       string
	Color      string
	IsTransfer bool
	Expense    Money
	Income     Money
	Count      int
}

// MerchantTotal is the aggregate of one merchant over a period.
type MerchantTotal struct {
	Merchant string
	Expense  Money
	Count    int
}

// DateRange is the span of dates that have transactions.
type DateRange struct {
	From    time.Time
	To      time.Time
	HasData bool
}

// TransactionFilter selects transactions for listing. ID narrows the list
// to one row so a receipt can jump straight to its operation.
type TransactionFilter struct {
	ID               *int64
	From             *time.Time
	To               *time.Time
	CategoryID       *int64
	Uncategorized    bool
	Direction        Direction
	Kind             Kind
	Query            string
	IncludeTransfers bool
	Limit            int
	Offset           int
}

// ListTotals sums every row a TransactionFilter matches, not just the
// page, so a filtered list can show what it adds up to. Excluded rows are
// left out, as everywhere else.
type ListTotals struct {
	Count   int
	Expense Money
	Income  Money
}

// TransactionPatch is a partial update. SetCategory distinguishes "leave
// the category alone" from "clear it".
type TransactionPatch struct {
	SetCategory bool
	CategoryID  *int64
	Note        *string
	Excluded    *bool
}

// CategoryChange is one row of a bulk re-categorization.
type CategoryChange struct {
	TransactionID int64
	CategoryID    *int64
	Source        CategorySource
}

// ImportResult reports what an upload did.
type ImportResult struct {
	Statement  Statement
	Imported   int
	Duplicates int
	Warnings   []string
}

// Budget is a monthly spending limit for one category, or for all spending
// when CategoryID is nil.
type Budget struct {
	ID         int64
	CategoryID *int64
	Amount     Money
}

// BudgetStatus is the progress of one budget within a month. Forecast
// extrapolates the spend so far to the whole month at the current pace;
// PaceLimit is the share of the limit that an even pace would have used
// by today, so "ahead of plan" can be shown before the limit is hit.
type BudgetStatus struct {
	BudgetID   int64
	CategoryID *int64
	Name       string
	Color      string
	Limit      Money
	Spent      Money
	Forecast   Money
	PaceLimit  Money
}

// BudgetReport is the month view of every budget plus the overall total.
type BudgetReport struct {
	Month       time.Time
	DaysInMonth int
	DaysElapsed int
	Items       []BudgetStatus
	Total       BudgetStatus
}

// Comparison sets a period against the previous one of the same length.
type Comparison struct {
	Current        RangeQuery
	Previous       RangeQuery
	CurrentTotals  Totals
	PreviousTotals Totals
	Categories     []CategoryDelta
	Merchants      []MerchantDelta
}

// CategoryDelta is the expense of one category in both periods.
type CategoryDelta struct {
	CategoryID *int64
	Name       string
	Color      string
	IsTransfer bool
	Current    Money
	Previous   Money
}

// MerchantDelta is the expense at one merchant in both periods.
type MerchantDelta struct {
	Merchant string
	Current  Money
	Previous Money
}
