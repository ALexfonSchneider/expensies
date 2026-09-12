package domain

import (
	"context"
	"time"
)

// StatementParser turns a bank statement file into transactions.
type StatementParser interface {
	Parse(ctx context.Context, data []byte) (*ParsedStatement, error)
}

// StatementRepository persists uploaded statements.
type StatementRepository interface {
	// Import stores the statement, its file and transactions atomically.
	// Transactions whose fingerprint already exists are skipped and counted
	// as duplicates. Returns ErrAlreadyExists when the same file or the same
	// bank statement number was imported before.
	Import(ctx context.Context, st *Statement, file []byte, txs []Transaction, now time.Time) (*ImportResult, error)
	List(ctx context.Context) ([]Statement, error)
	Get(ctx context.Context, id int64) (*Statement, error)
	Delete(ctx context.Context, id int64) error
}

// TransactionRepository reads and updates individual transactions.
type TransactionRepository interface {
	List(ctx context.Context, f TransactionFilter) ([]Transaction, ListTotals, error)
	Get(ctx context.Context, id int64) (*Transaction, error)
	Update(ctx context.Context, id int64, p TransactionPatch) error
	// ListAutoCategorized returns every transaction whose category was not
	// set manually, so rules can be re-applied without touching user edits.
	ListAutoCategorized(ctx context.Context) ([]Transaction, error)
	SetCategories(ctx context.Context, changes []CategoryChange) error
}

// CategoryRepository manages categories and their rules.
type CategoryRepository interface {
	List(ctx context.Context) ([]Category, error)
	Get(ctx context.Context, id int64) (*Category, error)
	Create(ctx context.Context, c *Category, now time.Time) error
	Update(ctx context.Context, c *Category) error
	Delete(ctx context.Context, id int64) error
	ListRules(ctx context.Context) ([]Rule, error)
	CreateRule(ctx context.Context, r *Rule, now time.Time) error
	DeleteRule(ctx context.Context, id int64) error
}

// AnalyticsRepository answers aggregate questions.
type AnalyticsRepository interface {
	Series(ctx context.Context, q SeriesQuery) ([]Bucket, error)
	Totals(ctx context.Context, q RangeQuery) (Totals, error)
	ByCategory(ctx context.Context, q RangeQuery) ([]CategoryTotal, error)
	TopMerchants(ctx context.Context, q RangeQuery, limit int) ([]MerchantTotal, error)
	DataRange(ctx context.Context) (DateRange, error)
}

// BudgetRepository stores monthly limits. Upsert replaces the limit of the
// same category (or the overall limit) and fills in the ID.
type BudgetRepository interface {
	List(ctx context.Context) ([]Budget, error)
	Upsert(ctx context.Context, b *Budget, now time.Time) error
	Delete(ctx context.Context, id int64) error
}
