package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/ALexfonSchneider/expenses/internal/domain"
)

const (
	defaultPageSize = 50
	maxPageSize     = 500
)

// ListTransactions returns a page of transactions and the totals of every
// row the filter matches.
func (s *Service) ListTransactions(ctx context.Context, f domain.TransactionFilter) ([]domain.Transaction, domain.ListTotals, error) {
	if f.Limit <= 0 {
		f.Limit = defaultPageSize
	}
	if f.Limit > maxPageSize {
		f.Limit = maxPageSize
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
	if f.From != nil && f.To != nil && f.From.After(*f.To) {
		return nil, domain.ListTotals{}, fmt.Errorf("%w: from is after to", domain.ErrInvalid)
	}
	switch f.Direction {
	case "", domain.DirectionExpense, domain.DirectionIncome:
	default:
		return nil, domain.ListTotals{}, fmt.Errorf("%w: unknown direction %q", domain.ErrInvalid, f.Direction)
	}
	items, totals, err := s.transactions.List(ctx, f)
	if err != nil {
		return nil, domain.ListTotals{}, fmt.Errorf("app: list transactions: %w", err)
	}
	return items, totals, nil
}

// UpdateTransaction applies a manual category or note change and returns
// the updated transaction.
func (s *Service) UpdateTransaction(ctx context.Context, id int64, p domain.TransactionPatch) (*domain.Transaction, error) {
	if p.SetCategory && p.CategoryID != nil {
		if _, err := s.categories.Get(ctx, *p.CategoryID); err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return nil, fmt.Errorf("%w: category %d does not exist", domain.ErrInvalid, *p.CategoryID)
			}
			return nil, fmt.Errorf("app: check category: %w", err)
		}
	}
	if err := s.transactions.Update(ctx, id, p); err != nil {
		return nil, fmt.Errorf("app: update transaction: %w", err)
	}
	tx, err := s.transactions.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("app: reload transaction: %w", err)
	}
	return tx, nil
}
