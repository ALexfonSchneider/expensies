package postgresrepo

import (
	"context"
	"fmt"
	"time"

	"github.com/ALexfonSchneider/expenses/internal/domain"
)

// List implements domain.BudgetRepository.
func (r *Budgets) List(ctx context.Context) ([]domain.Budget, error) {
	rows, err := r.q().Query(ctx, `SELECT id, category_id, amount FROM budgets ORDER BY category_id NULLS FIRST, id`)
	if err != nil {
		return nil, fmt.Errorf("postgresrepo: list budgets: %w", err)
	}
	defer rows.Close()

	out := make([]domain.Budget, 0)
	for rows.Next() {
		var b domain.Budget
		var amount int64
		if err := rows.Scan(&b.ID, &b.CategoryID, &amount); err != nil {
			return nil, fmt.Errorf("postgresrepo: scan budget: %w", err)
		}
		b.Amount = domain.Money(amount)
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgresrepo: list budgets: %w", err)
	}
	return out, nil
}

// Upsert implements domain.BudgetRepository.
func (r *Budgets) Upsert(ctx context.Context, b *domain.Budget, now time.Time) error {
	err := r.q().QueryRow(ctx, `
		INSERT INTO budgets (category_id, amount, created_at, updated_at)
		VALUES ($1, $2, $3, $3)
		ON CONFLICT (category_id) DO UPDATE SET amount = EXCLUDED.amount, updated_at = EXCLUDED.updated_at
		RETURNING id`,
		b.CategoryID, int64(b.Amount), now).Scan(&b.ID)
	if pgCode(err) == pgForeignKeyViolation {
		return fmt.Errorf("postgresrepo: upsert budget: category does not exist: %w", domain.ErrNotFound)
	}
	if err != nil {
		return fmt.Errorf("postgresrepo: upsert budget: %w", err)
	}
	return nil
}

// Delete implements domain.BudgetRepository.
func (r *Budgets) Delete(ctx context.Context, id int64) error {
	tag, err := r.q().Exec(ctx, `DELETE FROM budgets WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("postgresrepo: delete budget: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("postgresrepo: budget %d: %w", id, domain.ErrNotFound)
	}
	return nil
}
