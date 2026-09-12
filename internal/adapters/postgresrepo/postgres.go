// Package postgresrepo implements the domain repositories with pgx on top
// of the goplatform postgres component.
package postgresrepo

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ALexfonSchneider/goplatform/pkg/postgres"

	"github.com/ALexfonSchneider/expenses/internal/domain"
)

// base is shared by every repository type: one pool, one transaction
// helper. Separate types exist because the domain ports use the same
// method names (List, Get, Delete) with different signatures.
type base struct {
	db *postgres.DB
}

func (b base) q() querier {
	return b.db.Pool()
}

// Repository bundles the repository implementations built on one DB.
type Repository struct {
	Statements   *Statements
	Transactions *Transactions
	Categories   *Categories
	Analytics    *Analytics
	Budgets      *Budgets
	Settings     *Settings
	Receipts     *Receipts
}

// Statements implements domain.StatementRepository.
type Statements struct{ base }

// Transactions implements domain.TransactionRepository.
type Transactions struct{ base }

// Categories implements domain.CategoryRepository.
type Categories struct{ base }

// Analytics implements domain.AnalyticsRepository.
type Analytics struct{ base }

// Budgets implements domain.BudgetRepository.
type Budgets struct{ base }

// Settings implements domain.ReceiptSessionStore and domain.ReceiptSyncStore
// on top of a key/value table.
type Settings struct{ base }

// Receipts implements domain.ReceiptRepository.
type Receipts struct{ base }

// New creates the repositories. The DB must be started before queries run.
func New(db *postgres.DB) *Repository {
	b := base{db: db}
	return &Repository{
		Statements:   &Statements{b},
		Transactions: &Transactions{b},
		Categories:   &Categories{b},
		Analytics:    &Analytics{b},
		Budgets:      &Budgets{b},
		Settings:     &Settings{b},
		Receipts:     &Receipts{b},
	}
}

// querier is the subset of pgx shared by the pool and a transaction, so
// the same scan helpers serve both.
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// scanner is satisfied by both pgx.Row and pgx.Rows.
type scanner interface {
	Scan(dest ...any) error
}

const (
	pgUniqueViolation     = "23505"
	pgForeignKeyViolation = "23503"
)

func pgCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

// whereBuilder accumulates positional conditions for dynamic filters.
type whereBuilder struct {
	conds []string
	args  []any
}

// add appends a condition whose %d placeholders receive the next
// positional parameter number.
func (w *whereBuilder) add(cond string, arg any) {
	w.args = append(w.args, arg)
	w.conds = append(w.conds, fmt.Sprintf(cond, len(w.args)))
}

func (w *whereBuilder) raw(cond string) {
	w.conds = append(w.conds, cond)
}

func (w *whereBuilder) sql() string {
	if len(w.conds) == 0 {
		return ""
	}
	return " WHERE " + strings.Join(w.conds, " AND ")
}

// likePattern escapes user input for ILIKE and wraps it in wildcards.
func likePattern(s string) string {
	r := strings.NewReplacer(`\`, `\`, `%`, `\%`, `_`, `\_`)
	return "%" + r.Replace(s) + "%"
}

// Compile-time checks that the types satisfy the ports.
var (
	_ domain.StatementRepository   = (*Statements)(nil)
	_ domain.TransactionRepository = (*Transactions)(nil)
	_ domain.CategoryRepository    = (*Categories)(nil)
	_ domain.AnalyticsRepository   = (*Analytics)(nil)
	_ domain.BudgetRepository      = (*Budgets)(nil)
	_ domain.ReceiptSessionStore   = (*Settings)(nil)
	_ domain.ReceiptSyncStore      = (*Settings)(nil)
	_ domain.ReceiptRepository     = (*Receipts)(nil)
)
