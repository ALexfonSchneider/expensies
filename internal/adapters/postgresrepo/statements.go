package postgresrepo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ALexfonSchneider/expenses/internal/domain"
)

const statementColumns = `id, bank, number, period_from, period_to, opening_balance, closing_balance,
	total_income, total_expense, file_name, file_sha256, tx_count, uploaded_at`

func scanStatement(s scanner) (domain.Statement, error) {
	var st domain.Statement
	var opening, closing, income, expense int64
	err := s.Scan(&st.ID, &st.Bank, &st.Number, &st.PeriodFrom, &st.PeriodTo, &opening, &closing,
		&income, &expense, &st.FileName, &st.FileSHA256, &st.TxCount, &st.UploadedAt)
	if err != nil {
		return domain.Statement{}, err
	}
	st.OpeningBalance = domain.Money(opening)
	st.ClosingBalance = domain.Money(closing)
	st.TotalIncome = domain.Money(income)
	st.TotalExpense = domain.Money(expense)
	return st, nil
}

// Import implements domain.StatementRepository.
func (r *Statements) Import(ctx context.Context, st *domain.Statement, file []byte, txs []domain.Transaction, now time.Time) (*domain.ImportResult, error) {
	res := &domain.ImportResult{}
	err := r.db.WithTx(ctx, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			INSERT INTO statements (bank, number, period_from, period_to, opening_balance, closing_balance,
			                        total_income, total_expense, file_name, file_sha256, file_bytes, tx_count, uploaded_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
			RETURNING id`,
			st.Bank, st.Number, st.PeriodFrom, st.PeriodTo, int64(st.OpeningBalance), int64(st.ClosingBalance),
			int64(st.TotalIncome), int64(st.TotalExpense), st.FileName, st.FileSHA256, file, len(txs), now,
		).Scan(&st.ID)
		if pgCode(err) == pgUniqueViolation {
			return fmt.Errorf("postgresrepo: statement %s: %w", st.Number, domain.ErrAlreadyExists)
		}
		if err != nil {
			return fmt.Errorf("postgresrepo: insert statement: %w", err)
		}
		st.UploadedAt = now

		for i := range txs {
			t := &txs[i]
			tag, err := tx.Exec(ctx, `
				INSERT INTO transactions (statement_id, fingerprint, op_at, op_date, processed_on, card, amount,
				                          direction, kind, description, merchant, mcc, counterparty,
				                          category_id, category_source, note, created_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
				ON CONFLICT (fingerprint) DO NOTHING`,
				st.ID, t.Fingerprint, t.OpAt, domain.DateOf(t.OpAt), t.ProcessedOn, t.Card, int64(t.Amount),
				string(t.Direction), string(t.Kind), t.Description, t.Merchant, t.MCC, t.Counterparty,
				t.CategoryID, string(t.CategorySource), t.Note, now)
			if err != nil {
				return fmt.Errorf("postgresrepo: insert transaction: %w", err)
			}
			if tag.RowsAffected() == 1 {
				res.Imported++
			} else {
				res.Duplicates++
			}
		}

		// tx_count reflects rows attributed to this statement, not the
		// number of lines in the file, so overlapping uploads add up to the
		// real transaction count.
		if _, err := tx.Exec(ctx, `UPDATE statements SET tx_count = $1 WHERE id = $2`, res.Imported, st.ID); err != nil {
			return fmt.Errorf("postgresrepo: update tx_count: %w", err)
		}
		st.TxCount = res.Imported
		return nil
	})
	if err != nil {
		return nil, err
	}
	res.Statement = *st
	return res, nil
}

// List implements domain.StatementRepository.
func (r *Statements) List(ctx context.Context) ([]domain.Statement, error) {
	rows, err := r.q().Query(ctx, `SELECT `+statementColumns+` FROM statements ORDER BY period_from DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("postgresrepo: list statements: %w", err)
	}
	defer rows.Close()

	out := make([]domain.Statement, 0)
	for rows.Next() {
		st, err := scanStatement(rows)
		if err != nil {
			return nil, fmt.Errorf("postgresrepo: scan statement: %w", err)
		}
		out = append(out, st)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgresrepo: list statements: %w", err)
	}
	return out, nil
}

// Get implements domain.StatementRepository.
func (r *Statements) Get(ctx context.Context, id int64) (*domain.Statement, error) {
	st, err := scanStatement(r.q().QueryRow(ctx, `SELECT `+statementColumns+` FROM statements WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("postgresrepo: statement %d: %w", id, domain.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("postgresrepo: get statement: %w", err)
	}
	return &st, nil
}

// Delete implements domain.StatementRepository. Transactions go with the
// statement through ON DELETE CASCADE.
func (r *Statements) Delete(ctx context.Context, id int64) error {
	tag, err := r.q().Exec(ctx, `DELETE FROM statements WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("postgresrepo: delete statement: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("postgresrepo: statement %d: %w", id, domain.ErrNotFound)
	}
	return nil
}
