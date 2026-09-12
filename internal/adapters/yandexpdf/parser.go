// Package yandexpdf parses "Выписка по договору" PDF statements issued by
// Yandex Bank into domain transactions.
//
// The statement is a JasperReports table. Text is extracted per page with
// its coordinates and assigned to table columns by x position: the content
// stream interleaves wrapped description lines with the time column, so a
// plain text dump cannot be split back into rows reliably.
package yandexpdf

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dslipak/pdf"

	"github.com/ALexfonSchneider/expenses/internal/domain"
)

// Bank is the issuer identifier stored with every import.
const Bank = "yandex"

// Parser implements domain.StatementParser for Yandex Bank PDFs.
type Parser struct {
	loc *time.Location
}

// Option configures a Parser.
type Option func(*Parser)

// WithLocation sets the location of the times printed in the statement.
// Yandex Bank prints Moscow time, which is the default.
func WithLocation(loc *time.Location) Option {
	return func(p *Parser) {
		p.loc = loc
	}
}

// New creates a Parser.
func New(opts ...Option) (*Parser, error) {
	p := &Parser{loc: time.FixedZone("MSK", 3*60*60)}
	for _, opt := range opts {
		opt(p)
	}
	if p.loc == nil {
		return nil, fmt.Errorf("yandexpdf: location must not be nil")
	}
	return p, nil
}

// Parse implements domain.StatementParser.
func (p *Parser) Parse(ctx context.Context, data []byte) (ps *domain.ParsedStatement, err error) {
	// The rsc.io/pdf lineage panics on malformed input instead of returning
	// errors; a bad upload must become a 400, not a crashed server.
	defer func() {
		if r := recover(); r != nil {
			ps = nil
			err = fmt.Errorf("yandexpdf: %w: malformed pdf: %v", ErrUnsupported, r)
		}
	}()

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("yandexpdf: parse: %w", err)
	}
	reader, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("yandexpdf: %w: open pdf: %v", ErrUnsupported, err)
	}

	pages := make([][]row, 0, reader.NumPage())
	for n := 1; n <= reader.NumPage(); n++ {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("yandexpdf: parse: %w", err)
		}
		page := reader.Page(n)
		if page.V.IsNull() {
			continue
		}
		rows, err := page.GetTextByRow()
		if err != nil {
			return nil, fmt.Errorf("yandexpdf: page %d: extract text: %w", n, err)
		}
		pages = append(pages, convertRows(rows))
	}

	ps, err = parsePages(pages, p.loc)
	if err != nil {
		return nil, fmt.Errorf("yandexpdf: %w", err)
	}
	return ps, nil
}

// convertRows turns library rows into the internal representation, sorted
// top-to-bottom and left-to-right. PDF y grows upwards, so descending y is
// reading order.
func convertRows(rows pdf.Rows) []row {
	out := make([]row, 0, len(rows))
	for _, r := range rows {
		if r == nil {
			continue
		}
		items := make([]item, 0, len(r.Content))
		for _, t := range r.Content {
			s := strings.TrimSpace(t.S)
			if s == "" {
				continue
			}
			items = append(items, item{x: t.X, text: s})
		}
		if len(items) == 0 {
			continue
		}
		sort.SliceStable(items, func(i, j int) bool {
			return items[i].x < items[j].x
		})
		out = append(out, row{y: r.Position, items: items})
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].y > out[j].y
	})
	return out
}
