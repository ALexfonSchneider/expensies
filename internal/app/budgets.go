package app

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/ALexfonSchneider/expenses/internal/domain"
)

const totalBudgetName = "Все расходы"

// ListBudgets returns every monthly limit.
func (s *Service) ListBudgets(ctx context.Context) ([]domain.Budget, error) {
	items, err := s.budgets.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("app: list budgets: %w", err)
	}
	return items, nil
}

// SetBudget creates or replaces the monthly limit of a category (nil for
// the overall limit).
func (s *Service) SetBudget(ctx context.Context, categoryID *int64, amount domain.Money) (*domain.Budget, error) {
	if amount <= 0 {
		return nil, fmt.Errorf("%w: budget amount must be positive", domain.ErrInvalid)
	}
	if categoryID != nil {
		if _, err := s.categories.Get(ctx, *categoryID); err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return nil, fmt.Errorf("%w: category %d does not exist", domain.ErrInvalid, *categoryID)
			}
			return nil, fmt.Errorf("app: check category: %w", err)
		}
	}
	b := &domain.Budget{CategoryID: categoryID, Amount: amount}
	if err := s.budgets.Upsert(ctx, b, s.now()); err != nil {
		return nil, fmt.Errorf("app: set budget: %w", err)
	}
	return b, nil
}

// DeleteBudget removes a limit.
func (s *Service) DeleteBudget(ctx context.Context, id int64) error {
	if err := s.budgets.Delete(ctx, id); err != nil {
		return fmt.Errorf("app: delete budget: %w", err)
	}
	return nil
}

// BudgetReport measures every budget against the spending of one calendar
// month. Transfers never count towards budgets.
func (s *Service) BudgetReport(ctx context.Context, month time.Time) (*domain.BudgetReport, error) {
	start := domain.Date(month.Year(), month.Month(), 1)
	end := start.AddDate(0, 1, -1)
	daysInMonth := end.Day()

	today := domain.DateOf(s.now())
	var elapsed int
	switch {
	case today.Before(start):
		elapsed = 0
	case today.After(end):
		elapsed = daysInMonth
	default:
		elapsed = today.Day()
	}

	budgets, err := s.budgets.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("app: list budgets: %w", err)
	}
	categories, err := s.categories.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("app: list categories: %w", err)
	}
	q := domain.RangeQuery{From: start, To: end}
	byCategory, err := s.analytics.ByCategory(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("app: budget spending: %w", err)
	}
	totals, err := s.analytics.Totals(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("app: budget totals: %w", err)
	}

	catByID := make(map[int64]domain.Category, len(categories))
	for _, c := range categories {
		catByID[c.ID] = c
	}
	spentByCategory := make(map[int64]domain.Money, len(byCategory))
	for _, ct := range byCategory {
		if ct.CategoryID != nil {
			spentByCategory[*ct.CategoryID] = ct.Expense
		}
	}

	report := &domain.BudgetReport{
		Month:       start,
		DaysInMonth: daysInMonth,
		DaysElapsed: elapsed,
		Items:       make([]domain.BudgetStatus, 0, len(budgets)),
		Total: domain.BudgetStatus{
			Name:     totalBudgetName,
			Spent:    totals.Expense,
			Forecast: forecast(totals.Expense, elapsed, daysInMonth),
		},
	}
	for _, b := range budgets {
		if b.CategoryID == nil {
			report.Total.BudgetID = b.ID
			report.Total.Limit = b.Amount
			report.Total.PaceLimit = paceLimit(b.Amount, elapsed, daysInMonth)
			continue
		}
		cat, ok := catByID[*b.CategoryID]
		if !ok {
			// The category vanished between the two reads; the cascade
			// removes the budget, nothing to show.
			continue
		}
		spent := spentByCategory[*b.CategoryID]
		report.Items = append(report.Items, domain.BudgetStatus{
			BudgetID:   b.ID,
			CategoryID: b.CategoryID,
			Name:       cat.Name,
			Color:      cat.Color,
			Limit:      b.Amount,
			Spent:      spent,
			Forecast:   forecast(spent, elapsed, daysInMonth),
			PaceLimit:  paceLimit(b.Amount, elapsed, daysInMonth),
		})
	}
	// Most exhausted budgets first: that is what needs attention.
	sort.SliceStable(report.Items, func(i, j int) bool {
		a, b := report.Items[i], report.Items[j]
		return float64(a.Spent)/float64(a.Limit) > float64(b.Spent)/float64(b.Limit)
	})
	return report, nil
}

// forecast extrapolates the spend of the elapsed days to the full month.
// A month that has not started projects nothing; a finished month is its
// own forecast.
func forecast(spent domain.Money, elapsed, days int) domain.Money {
	switch {
	case elapsed <= 0:
		return 0
	case elapsed >= days:
		return spent
	default:
		return spent * domain.Money(days) / domain.Money(elapsed)
	}
}

// paceLimit is the share of the limit an even daily pace would have used.
func paceLimit(limit domain.Money, elapsed, days int) domain.Money {
	if days <= 0 {
		return limit
	}
	return limit * domain.Money(elapsed) / domain.Money(days)
}
