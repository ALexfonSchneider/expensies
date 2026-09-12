package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ALexfonSchneider/expenses/internal/domain"
)

const defaultCategoryColor = "#9e9e9e"

// ListCategories returns categories in display order.
func (s *Service) ListCategories(ctx context.Context) ([]domain.Category, error) {
	items, err := s.categories.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("app: list categories: %w", err)
	}
	return items, nil
}

// CreateCategory validates and stores a category, filling in its ID.
func (s *Service) CreateCategory(ctx context.Context, c *domain.Category) error {
	if err := normalizeCategory(c); err != nil {
		return err
	}
	if err := s.categories.Create(ctx, c, s.now()); err != nil {
		return fmt.Errorf("app: create category: %w", err)
	}
	return nil
}

// UpdateCategory replaces the editable fields of a category.
func (s *Service) UpdateCategory(ctx context.Context, c *domain.Category) error {
	if err := normalizeCategory(c); err != nil {
		return err
	}
	if err := s.categories.Update(ctx, c); err != nil {
		return fmt.Errorf("app: update category: %w", err)
	}
	return nil
}

// DeleteCategory removes a category; its transactions become uncategorized.
func (s *Service) DeleteCategory(ctx context.Context, id int64) error {
	if err := s.categories.Delete(ctx, id); err != nil {
		return fmt.Errorf("app: delete category: %w", err)
	}
	return nil
}

func normalizeCategory(c *domain.Category) error {
	c.Name = strings.TrimSpace(c.Name)
	if c.Name == "" {
		return fmt.Errorf("%w: category name is empty", domain.ErrInvalid)
	}
	c.Color = strings.TrimSpace(c.Color)
	if c.Color == "" {
		c.Color = defaultCategoryColor
	}
	return nil
}

// ListRules returns rules in evaluation order.
func (s *Service) ListRules(ctx context.Context) ([]domain.Rule, error) {
	items, err := s.categories.ListRules(ctx)
	if err != nil {
		return nil, fmt.Errorf("app: list rules: %w", err)
	}
	return items, nil
}

// CreateRule validates and stores a rule, filling in its ID.
func (s *Service) CreateRule(ctx context.Context, r *domain.Rule) error {
	r.Value = strings.TrimSpace(r.Value)
	if err := ValidateRule(*r); err != nil {
		return err
	}
	if _, err := s.categories.Get(ctx, r.CategoryID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("%w: category %d does not exist", domain.ErrInvalid, r.CategoryID)
		}
		return fmt.Errorf("app: check category: %w", err)
	}
	if err := s.categories.CreateRule(ctx, r, s.now()); err != nil {
		return fmt.Errorf("app: create rule: %w", err)
	}
	return nil
}

// DeleteRule removes a rule. Already categorized transactions keep their
// category until ApplyRules runs.
func (s *Service) DeleteRule(ctx context.Context, id int64) error {
	if err := s.categories.DeleteRule(ctx, id); err != nil {
		return fmt.Errorf("app: delete rule: %w", err)
	}
	return nil
}

// ApplyRules re-evaluates the rules for every transaction that was not
// categorized by hand and returns how many rows changed.
func (s *Service) ApplyRules(ctx context.Context) (int, error) {
	cat, err := s.categorizer(ctx)
	if err != nil {
		return 0, err
	}
	txs, err := s.transactions.ListAutoCategorized(ctx)
	if err != nil {
		return 0, fmt.Errorf("app: list transactions for rules: %w", err)
	}

	changes := make([]domain.CategoryChange, 0)
	for i := range txs {
		tx := &txs[i]
		var want *int64
		source := domain.CategorySourceNone
		if id, ok := cat.Match(tx); ok {
			want = &id
			source = domain.CategorySourceRule
		}
		if sameCategory(tx.CategoryID, want) && tx.CategorySource == source {
			continue
		}
		changes = append(changes, domain.CategoryChange{TransactionID: tx.ID, CategoryID: want, Source: source})
	}
	if len(changes) == 0 {
		return 0, nil
	}
	if err := s.transactions.SetCategories(ctx, changes); err != nil {
		return 0, fmt.Errorf("app: apply rules: %w", err)
	}
	s.logger.InfoContext(ctx, "rules applied", "changed", len(changes), "checked", len(txs))
	return len(changes), nil
}

func sameCategory(a, b *int64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func (s *Service) categorizer(ctx context.Context) (*Categorizer, error) {
	rules, err := s.categories.ListRules(ctx)
	if err != nil {
		return nil, fmt.Errorf("app: list rules: %w", err)
	}
	cat, err := NewCategorizer(rules)
	if err != nil {
		return nil, fmt.Errorf("app: compile rules: %w", err)
	}
	return cat, nil
}
