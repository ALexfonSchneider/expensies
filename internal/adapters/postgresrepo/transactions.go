package postgresrepo

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/ALexfonSchneider/expenses/internal/domain"
)

const txColumns = `t.id, t.statement_id, t.source, t.fingerprint, t.op_at, t.processed_on, t.card, t.amount,
	t.direction, t.kind, t.description, t.merchant, t.mcc, t.counterparty, t.category_id, t.category_source, t.note,
	t.excluded, (SELECT min(rc.id) FROM receipts rc WHERE rc.transaction_id = t.id)`

func scanTransaction(s scanner) (domain.Transaction, error) {
	var t domain.Transaction
	var statementID *int64
	var amount int64
	var origin, direction, kind, source string
	err := s.Scan(&t.ID, &statementID, &origin, &t.Fingerprint, &t.OpAt, &t.ProcessedOn, &t.Card, &amount,
		&direction, &kind, &t.Description, &t.Merchant, &t.MCC, &t.Counterparty, &t.CategoryID, &source, &t.Note,
		&t.Excluded, &t.ReceiptID)
	if err != nil {
		return domain.Transaction{}, err
	}
	if statementID != nil {
		t.StatementID = *statementID
	}
	t.Source = domain.TransactionSource(origin)
	t.Amount = domain.Money(amount)
	t.Direction = domain.Direction(direction)
	t.Kind = domain.Kind(kind)
	t.CategorySource = domain.CategorySource(source)
	return t, nil
}

func collectTransactions(rows pgx.Rows) ([]domain.Transaction, error) {
	defer rows.Close()
	out := make([]domain.Transaction, 0)
	for rows.Next() {
		t, err := scanTransaction(rows)
		if err != nil {
			return nil, fmt.Errorf("postgresrepo: scan transaction: %w", err)
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgresrepo: read transactions: %w", err)
	}
	return out, nil
}

func transactionWhere(f domain.TransactionFilter) *whereBuilder {
	w := &whereBuilder{}
	if f.ID != nil {
		w.add("t.id = $%d", *f.ID)
	}
	if f.From != nil {
		w.add("t.op_date >= $%d", *f.From)
	}
	if f.To != nil {
		w.add("t.op_date <= $%d", *f.To)
	}
	switch {
	case f.Uncategorized:
		w.raw("t.category_id IS NULL")
	case f.CategoryID != nil:
		w.add("t.category_id = $%d", *f.CategoryID)
	}
	if f.Direction != "" {
		w.add("t.direction = $%d", string(f.Direction))
	}
	if f.Kind != "" {
		w.add("t.kind = $%d", string(f.Kind))
	}
	if q := strings.TrimSpace(f.Query); q != "" {
		w.add("(t.description ILIKE $%[1]d OR t.merchant ILIKE $%[1]d OR t.counterparty ILIKE $%[1]d OR t.note ILIKE $%[1]d)", likePattern(q))
	}
	if !f.IncludeTransfers {
		w.raw("COALESCE(c.is_transfer, FALSE) = FALSE")
	}
	return w
}

// List implements domain.TransactionRepository.
func (r *Transactions) List(ctx context.Context, f domain.TransactionFilter) ([]domain.Transaction, domain.ListTotals, error) {
	w := transactionWhere(f)
	from := ` FROM transactions t LEFT JOIN categories c ON c.id = t.category_id` + w.sql()

	var totals domain.ListTotals
	var expense, income int64
	err := r.q().QueryRow(ctx, `
		SELECT count(*),
		       COALESCE(SUM(t.amount) FILTER (WHERE t.direction = 'expense' AND NOT t.excluded), 0),
		       COALESCE(SUM(t.amount) FILTER (WHERE t.direction = 'income' AND NOT t.excluded), 0)`+from,
		w.args...).Scan(&totals.Count, &expense, &income)
	if err != nil {
		return nil, domain.ListTotals{}, fmt.Errorf("postgresrepo: count transactions: %w", err)
	}
	totals.Expense = domain.Money(expense)
	totals.Income = domain.Money(income)

	args := append(append([]any{}, w.args...), f.Limit, f.Offset)
	rows, err := r.q().Query(ctx,
		`SELECT `+txColumns+from+fmt.Sprintf(` ORDER BY t.op_at DESC, t.id DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args)),
		args...)
	if err != nil {
		return nil, domain.ListTotals{}, fmt.Errorf("postgresrepo: list transactions: %w", err)
	}
	items, err := collectTransactions(rows)
	if err != nil {
		return nil, domain.ListTotals{}, err
	}
	return items, totals, nil
}

// Get implements domain.TransactionRepository.
func (r *Transactions) Get(ctx context.Context, id int64) (*domain.Transaction, error) {
	t, err := scanTransaction(r.q().QueryRow(ctx, `SELECT `+txColumns+` FROM transactions t WHERE t.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("postgresrepo: transaction %d: %w", id, domain.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("postgresrepo: get transaction: %w", err)
	}
	return &t, nil
}

// Update implements domain.TransactionRepository.
func (r *Transactions) Update(ctx context.Context, id int64, p domain.TransactionPatch) error {
	sets := make([]string, 0, 3)
	args := make([]any, 0, 4)
	if p.SetCategory {
		source := domain.CategorySourceNone
		if p.CategoryID != nil {
			source = domain.CategorySourceManual
		}
		args = append(args, p.CategoryID)
		sets = append(sets, fmt.Sprintf("category_id = $%d", len(args)))
		args = append(args, string(source))
		sets = append(sets, fmt.Sprintf("category_source = $%d", len(args)))
	}
	if p.Note != nil {
		args = append(args, *p.Note)
		sets = append(sets, fmt.Sprintf("note = $%d", len(args)))
	}
	if p.Excluded != nil {
		args = append(args, *p.Excluded)
		sets = append(sets, fmt.Sprintf("excluded = $%d", len(args)))
	}
	if len(sets) == 0 {
		return nil
	}
	args = append(args, id)
	tag, err := r.q().Exec(ctx,
		fmt.Sprintf(`UPDATE transactions SET %s WHERE id = $%d`, strings.Join(sets, ", "), len(args)),
		args...)
	if pgCode(err) == pgForeignKeyViolation {
		return fmt.Errorf("postgresrepo: update transaction: category does not exist: %w", domain.ErrInvalid)
	}
	if err != nil {
		return fmt.Errorf("postgresrepo: update transaction: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("postgresrepo: transaction %d: %w", id, domain.ErrNotFound)
	}
	return nil
}

// ListAutoCategorized implements domain.TransactionRepository.
func (r *Transactions) ListAutoCategorized(ctx context.Context) ([]domain.Transaction, error) {
	rows, err := r.q().Query(ctx,
		`SELECT `+txColumns+` FROM transactions t WHERE t.category_source <> $1 ORDER BY t.id`,
		string(domain.CategorySourceManual))
	if err != nil {
		return nil, fmt.Errorf("postgresrepo: list auto-categorized: %w", err)
	}
	return collectTransactions(rows)
}

// SetCategories implements domain.TransactionRepository.
func (r *Transactions) SetCategories(ctx context.Context, changes []domain.CategoryChange) error {
	if len(changes) == 0 {
		return nil
	}
	return r.db.WithTx(ctx, func(tx pgx.Tx) error {
		batch := &pgx.Batch{}
		for _, ch := range changes {
			batch.Queue(`UPDATE transactions SET category_id = $1, category_source = $2 WHERE id = $3`,
				ch.CategoryID, string(ch.Source), ch.TransactionID)
		}
		results := tx.SendBatch(ctx, batch)
		for range changes {
			if _, err := results.Exec(); err != nil {
				// The batch already failed and WithTx rolls the transaction
				// back; a second error from Close adds nothing.
				_ = results.Close()
				return fmt.Errorf("postgresrepo: set categories: %w", err)
			}
		}
		if err := results.Close(); err != nil {
			return fmt.Errorf("postgresrepo: set categories: %w", err)
		}
		return nil
	})
}
