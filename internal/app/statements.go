package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/ALexfonSchneider/expenses/internal/domain"
)

// ImportStatement parses an uploaded statement, categorizes its lines with
// the current rules and stores everything atomically.
func (s *Service) ImportStatement(ctx context.Context, fileName string, data []byte) (*domain.ImportResult, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("%w: empty file", domain.ErrInvalid)
	}
	now := s.now()

	ps, err := s.parser.Parse(ctx, data)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("app: parse statement: %w", err)
		}
		// Anything the parser rejects is a problem with the file, not with
		// the server, so it surfaces as a client error with the reason.
		return nil, fmt.Errorf("%w: %v", domain.ErrInvalid, err)
	}

	sum := sha256.Sum256(data)
	st := ps.Statement
	st.FileName = fileName
	st.FileSHA256 = hex.EncodeToString(sum[:])

	cat, err := s.categorizer(ctx)
	if err != nil {
		return nil, err
	}
	for i := range ps.Transactions {
		tx := &ps.Transactions[i]
		if id, ok := cat.Match(tx); ok {
			tx.CategoryID = &id
			tx.CategorySource = domain.CategorySourceRule
		}
	}

	res, err := s.statements.Import(ctx, &st, data, ps.Transactions, now)
	if err != nil {
		return nil, fmt.Errorf("app: import statement: %w", err)
	}
	res.Warnings = ps.Warnings

	// Receipts that arrived before their statement are waiting for exactly
	// these lines; a failure here must not undo a successful import.
	if matched, err := s.MatchReceipts(ctx); err != nil {
		s.logger.WarnContext(ctx, "match receipts after import", "error", err)
	} else if matched > 0 {
		s.logger.InfoContext(ctx, "receipts matched after import", "matched", matched)
	}

	s.logger.InfoContext(ctx, "statement imported",
		"number", st.Number, "period_from", st.PeriodFrom.Format("2006-01-02"), "period_to", st.PeriodTo.Format("2006-01-02"),
		"imported", res.Imported, "duplicates", res.Duplicates, "warnings", len(ps.Warnings))
	return res, nil
}

// ListStatements returns uploaded statements, newest period first.
func (s *Service) ListStatements(ctx context.Context) ([]domain.Statement, error) {
	items, err := s.statements.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("app: list statements: %w", err)
	}
	return items, nil
}

// DeleteStatement removes a statement together with its transactions.
func (s *Service) DeleteStatement(ctx context.Context, id int64) error {
	if err := s.statements.Delete(ctx, id); err != nil {
		return fmt.Errorf("app: delete statement: %w", err)
	}
	s.logger.InfoContext(ctx, "statement deleted", "id", id)
	return nil
}
