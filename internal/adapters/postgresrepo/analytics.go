package postgresrepo

import (
	"context"
	"fmt"
	"time"

	"github.com/ALexfonSchneider/expenses/internal/domain"
)

// rangeFrom is the shared FROM/WHERE of every aggregate: the category join
// is needed to drop transfer categories unless the caller opts in, and
// lines the user excluded never count.
const rangeFrom = ` FROM transactions t LEFT JOIN categories c ON c.id = t.category_id
	WHERE t.op_date BETWEEN $1 AND $2 AND NOT t.excluded
	  AND ($3::boolean OR COALESCE(c.is_transfer, FALSE) = FALSE)`

// Series implements domain.AnalyticsRepository. Gaps are not filled here;
// the application layer adds zero buckets so the SQL stays simple.
func (r *Analytics) Series(ctx context.Context, q domain.SeriesQuery) ([]domain.Bucket, error) {
	switch q.Granularity {
	case domain.GranularityDay, domain.GranularityWeek, domain.GranularityMonth:
	default:
		return nil, fmt.Errorf("postgresrepo: series: granularity %q: %w", q.Granularity, domain.ErrInvalid)
	}
	rows, err := r.q().Query(ctx, `
		SELECT date_trunc($4, t.op_date::timestamp)::date AS bucket,
		       COALESCE(SUM(t.amount) FILTER (WHERE t.direction = 'expense'), 0),
		       COALESCE(SUM(t.amount) FILTER (WHERE t.direction = 'income'), 0),
		       COUNT(*)`+rangeFrom+`
		GROUP BY 1 ORDER BY 1`,
		q.From, q.To, q.IncludeTransfers, string(q.Granularity))
	if err != nil {
		return nil, fmt.Errorf("postgresrepo: series: %w", err)
	}
	defer rows.Close()

	out := make([]domain.Bucket, 0)
	for rows.Next() {
		var b domain.Bucket
		var expense, income int64
		if err := rows.Scan(&b.Start, &expense, &income, &b.Count); err != nil {
			return nil, fmt.Errorf("postgresrepo: scan bucket: %w", err)
		}
		b.Expense = domain.Money(expense)
		b.Income = domain.Money(income)
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgresrepo: series: %w", err)
	}
	return out, nil
}

// Totals implements domain.AnalyticsRepository.
func (r *Analytics) Totals(ctx context.Context, q domain.RangeQuery) (domain.Totals, error) {
	var t domain.Totals
	var expense, income int64
	err := r.q().QueryRow(ctx, `
		SELECT COALESCE(SUM(t.amount) FILTER (WHERE t.direction = 'expense'), 0),
		       COALESCE(SUM(t.amount) FILTER (WHERE t.direction = 'income'), 0),
		       COUNT(*)`+rangeFrom,
		q.From, q.To, q.IncludeTransfers).Scan(&expense, &income, &t.Count)
	if err != nil {
		return domain.Totals{}, fmt.Errorf("postgresrepo: totals: %w", err)
	}
	t.Expense = domain.Money(expense)
	t.Income = domain.Money(income)
	return t, nil
}

// ByCategory implements domain.AnalyticsRepository.
func (r *Analytics) ByCategory(ctx context.Context, q domain.RangeQuery) ([]domain.CategoryTotal, error) {
	rows, err := r.q().Query(ctx, `
		SELECT c.id, COALESCE(c.name, ''), COALESCE(c.color, ''), COALESCE(c.is_transfer, FALSE),
		       COALESCE(SUM(t.amount) FILTER (WHERE t.direction = 'expense'), 0),
		       COALESCE(SUM(t.amount) FILTER (WHERE t.direction = 'income'), 0),
		       COUNT(*)`+rangeFrom+`
		GROUP BY c.id, c.name, c.color, c.is_transfer
		ORDER BY 5 DESC, 6 DESC`,
		q.From, q.To, q.IncludeTransfers)
	if err != nil {
		return nil, fmt.Errorf("postgresrepo: by category: %w", err)
	}
	defer rows.Close()

	out := make([]domain.CategoryTotal, 0)
	for rows.Next() {
		var ct domain.CategoryTotal
		var expense, income int64
		if err := rows.Scan(&ct.CategoryID, &ct.Name, &ct.Color, &ct.IsTransfer, &expense, &income, &ct.Count); err != nil {
			return nil, fmt.Errorf("postgresrepo: scan category total: %w", err)
		}
		ct.Expense = domain.Money(expense)
		ct.Income = domain.Money(income)
		out = append(out, ct)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgresrepo: by category: %w", err)
	}
	return out, nil
}

// TopMerchants implements domain.AnalyticsRepository. Lines without a
// merchant group by counterparty, then by description.
func (r *Analytics) TopMerchants(ctx context.Context, q domain.RangeQuery, limit int) ([]domain.MerchantTotal, error) {
	rows, err := r.q().Query(ctx, `
		SELECT COALESCE(NULLIF(t.merchant, ''), NULLIF(t.counterparty, ''), t.description) AS name,
		       SUM(t.amount), COUNT(*)`+rangeFrom+`
		  AND t.direction = 'expense'
		GROUP BY 1 ORDER BY 2 DESC LIMIT $4`,
		q.From, q.To, q.IncludeTransfers, limit)
	if err != nil {
		return nil, fmt.Errorf("postgresrepo: top merchants: %w", err)
	}
	defer rows.Close()

	out := make([]domain.MerchantTotal, 0)
	for rows.Next() {
		var mt domain.MerchantTotal
		var expense int64
		if err := rows.Scan(&mt.Merchant, &expense, &mt.Count); err != nil {
			return nil, fmt.Errorf("postgresrepo: scan merchant total: %w", err)
		}
		mt.Expense = domain.Money(expense)
		out = append(out, mt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgresrepo: top merchants: %w", err)
	}
	return out, nil
}

// DataRange implements domain.AnalyticsRepository.
func (r *Analytics) DataRange(ctx context.Context) (domain.DateRange, error) {
	var from, to *time.Time
	if err := r.q().QueryRow(ctx, `SELECT MIN(op_date), MAX(op_date) FROM transactions`).Scan(&from, &to); err != nil {
		return domain.DateRange{}, fmt.Errorf("postgresrepo: data range: %w", err)
	}
	if from == nil || to == nil {
		return domain.DateRange{}, nil
	}
	return domain.DateRange{From: *from, To: *to, HasData: true}, nil
}
