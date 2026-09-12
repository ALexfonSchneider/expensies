package postgresrepo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ALexfonSchneider/expenses/internal/domain"
)

const categoryColumns = `id, name, color, is_transfer, sort_order`

func scanCategory(s scanner) (domain.Category, error) {
	var c domain.Category
	err := s.Scan(&c.ID, &c.Name, &c.Color, &c.IsTransfer, &c.SortOrder)
	return c, err
}

// List implements domain.CategoryRepository.
func (r *Categories) List(ctx context.Context) ([]domain.Category, error) {
	rows, err := r.q().Query(ctx, `SELECT `+categoryColumns+` FROM categories ORDER BY sort_order, name`)
	if err != nil {
		return nil, fmt.Errorf("postgresrepo: list categories: %w", err)
	}
	defer rows.Close()

	out := make([]domain.Category, 0)
	for rows.Next() {
		c, err := scanCategory(rows)
		if err != nil {
			return nil, fmt.Errorf("postgresrepo: scan category: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgresrepo: list categories: %w", err)
	}
	return out, nil
}

// Get implements domain.CategoryRepository.
func (r *Categories) Get(ctx context.Context, id int64) (*domain.Category, error) {
	c, err := scanCategory(r.q().QueryRow(ctx, `SELECT `+categoryColumns+` FROM categories WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("postgresrepo: category %d: %w", id, domain.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("postgresrepo: get category: %w", err)
	}
	return &c, nil
}

// Create implements domain.CategoryRepository.
func (r *Categories) Create(ctx context.Context, c *domain.Category, now time.Time) error {
	err := r.q().QueryRow(ctx, `
		INSERT INTO categories (name, color, is_transfer, sort_order, created_at)
		VALUES ($1, $2, $3, $4, $5) RETURNING id`,
		c.Name, c.Color, c.IsTransfer, c.SortOrder, now).Scan(&c.ID)
	if pgCode(err) == pgUniqueViolation {
		return fmt.Errorf("postgresrepo: category %q: %w", c.Name, domain.ErrAlreadyExists)
	}
	if err != nil {
		return fmt.Errorf("postgresrepo: create category: %w", err)
	}
	return nil
}

// Update implements domain.CategoryRepository.
func (r *Categories) Update(ctx context.Context, c *domain.Category) error {
	tag, err := r.q().Exec(ctx, `
		UPDATE categories SET name = $1, color = $2, is_transfer = $3, sort_order = $4 WHERE id = $5`,
		c.Name, c.Color, c.IsTransfer, c.SortOrder, c.ID)
	if pgCode(err) == pgUniqueViolation {
		return fmt.Errorf("postgresrepo: category %q: %w", c.Name, domain.ErrAlreadyExists)
	}
	if err != nil {
		return fmt.Errorf("postgresrepo: update category: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("postgresrepo: category %d: %w", c.ID, domain.ErrNotFound)
	}
	return nil
}

// Delete implements domain.CategoryRepository. Rules cascade and
// transactions fall back to uncategorized through the foreign keys.
func (r *Categories) Delete(ctx context.Context, id int64) error {
	tag, err := r.q().Exec(ctx, `DELETE FROM categories WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("postgresrepo: delete category: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("postgresrepo: category %d: %w", id, domain.ErrNotFound)
	}
	return nil
}

// ListRules implements domain.CategoryRepository.
func (r *Categories) ListRules(ctx context.Context) ([]domain.Rule, error) {
	rows, err := r.q().Query(ctx,
		`SELECT id, category_id, field, op, value, priority FROM category_rules ORDER BY priority, id`)
	if err != nil {
		return nil, fmt.Errorf("postgresrepo: list rules: %w", err)
	}
	defer rows.Close()

	out := make([]domain.Rule, 0)
	for rows.Next() {
		var rule domain.Rule
		var field, op string
		if err := rows.Scan(&rule.ID, &rule.CategoryID, &field, &op, &rule.Value, &rule.Priority); err != nil {
			return nil, fmt.Errorf("postgresrepo: scan rule: %w", err)
		}
		rule.Field = domain.RuleField(field)
		rule.Op = domain.RuleOp(op)
		out = append(out, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgresrepo: list rules: %w", err)
	}
	return out, nil
}

// CreateRule implements domain.CategoryRepository.
func (r *Categories) CreateRule(ctx context.Context, rule *domain.Rule, now time.Time) error {
	err := r.q().QueryRow(ctx, `
		INSERT INTO category_rules (category_id, field, op, value, priority, created_at)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		rule.CategoryID, string(rule.Field), string(rule.Op), rule.Value, rule.Priority, now).Scan(&rule.ID)
	if pgCode(err) == pgForeignKeyViolation {
		return fmt.Errorf("postgresrepo: create rule: category %d: %w", rule.CategoryID, domain.ErrNotFound)
	}
	if err != nil {
		return fmt.Errorf("postgresrepo: create rule: %w", err)
	}
	return nil
}

// DeleteRule implements domain.CategoryRepository.
func (r *Categories) DeleteRule(ctx context.Context, id int64) error {
	tag, err := r.q().Exec(ctx, `DELETE FROM category_rules WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("postgresrepo: delete rule: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("postgresrepo: rule %d: %w", id, domain.ErrNotFound)
	}
	return nil
}
