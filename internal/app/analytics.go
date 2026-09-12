package app

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/ALexfonSchneider/expenses/internal/domain"
)

const (
	maxRangeDays        = 366 * 10
	defaultMerchantsTop = 10
	maxMerchantsTop     = 100
)

// Summary builds the dashboard headline for a period, including the
// previous period of the same length for trends.
func (s *Service) Summary(ctx context.Context, q domain.RangeQuery) (*domain.Summary, error) {
	if err := validateRange(q); err != nil {
		return nil, err
	}
	totals, err := s.analytics.Totals(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("app: summary totals: %w", err)
	}
	days, err := s.analytics.Series(ctx, domain.SeriesQuery{RangeQuery: q, Granularity: domain.GranularityDay})
	if err != nil {
		return nil, fmt.Errorf("app: summary days: %w", err)
	}

	length := daysBetween(q.From, q.To)
	prev := previousRange(q)
	prevTotals, err := s.analytics.Totals(ctx, prev)
	if err != nil {
		return nil, fmt.Errorf("app: summary previous period: %w", err)
	}

	sum := &domain.Summary{
		From:    q.From,
		To:      q.To,
		Expense: totals.Expense,
		Income:  totals.Income,
		Count:   totals.Count,
		Days:    length,
		Prev:    prevTotals,
	}
	for _, b := range days {
		if b.Expense <= 0 {
			continue
		}
		sum.ActiveDays++
		if sum.MaxDay == nil || b.Expense > sum.MaxDay.Expense {
			day := b
			sum.MaxDay = &day
		}
	}
	if length > 0 {
		sum.AvgPerDay = totals.Expense / domain.Money(length)
	}
	if sum.ActiveDays > 0 {
		sum.AvgPerActiveDay = totals.Expense / domain.Money(sum.ActiveDays)
	}
	return sum, nil
}

// Series returns expense and income per bucket with empty buckets filled
// in, so charts show gaps as zeros instead of skipping them.
func (s *Service) Series(ctx context.Context, q domain.SeriesQuery) ([]domain.Bucket, error) {
	if err := validateRange(q.RangeQuery); err != nil {
		return nil, err
	}
	switch q.Granularity {
	case domain.GranularityDay, domain.GranularityWeek, domain.GranularityMonth:
	default:
		return nil, fmt.Errorf("%w: unknown granularity %q", domain.ErrInvalid, q.Granularity)
	}
	raw, err := s.analytics.Series(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("app: series: %w", err)
	}
	return fillGaps(raw, q.From, q.To, q.Granularity), nil
}

// ByCategory returns per-category totals for a period.
func (s *Service) ByCategory(ctx context.Context, q domain.RangeQuery) ([]domain.CategoryTotal, error) {
	if err := validateRange(q); err != nil {
		return nil, err
	}
	items, err := s.analytics.ByCategory(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("app: by category: %w", err)
	}
	return items, nil
}

// TopMerchants returns the merchants with the biggest spend in a period.
func (s *Service) TopMerchants(ctx context.Context, q domain.RangeQuery, limit int) ([]domain.MerchantTotal, error) {
	if err := validateRange(q); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = defaultMerchantsTop
	}
	if limit > maxMerchantsTop {
		limit = maxMerchantsTop
	}
	items, err := s.analytics.TopMerchants(ctx, q, limit)
	if err != nil {
		return nil, fmt.Errorf("app: top merchants: %w", err)
	}
	return items, nil
}

// Compare sets the period against the previous one of the same length,
// per category and per merchant, biggest movements first.
func (s *Service) Compare(ctx context.Context, q domain.RangeQuery) (*domain.Comparison, error) {
	if err := validateRange(q); err != nil {
		return nil, err
	}
	prev := previousRange(q)

	curTotals, err := s.analytics.Totals(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("app: compare totals: %w", err)
	}
	prevTotals, err := s.analytics.Totals(ctx, prev)
	if err != nil {
		return nil, fmt.Errorf("app: compare previous totals: %w", err)
	}
	curCats, err := s.analytics.ByCategory(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("app: compare categories: %w", err)
	}
	prevCats, err := s.analytics.ByCategory(ctx, prev)
	if err != nil {
		return nil, fmt.Errorf("app: compare previous categories: %w", err)
	}
	curMerchants, err := s.analytics.TopMerchants(ctx, q, maxMerchantsTop)
	if err != nil {
		return nil, fmt.Errorf("app: compare merchants: %w", err)
	}
	prevMerchants, err := s.analytics.TopMerchants(ctx, prev, maxMerchantsTop)
	if err != nil {
		return nil, fmt.Errorf("app: compare previous merchants: %w", err)
	}

	return &domain.Comparison{
		Current:        q,
		Previous:       prev,
		CurrentTotals:  curTotals,
		PreviousTotals: prevTotals,
		Categories:     mergeCategoryDeltas(curCats, prevCats),
		Merchants:      mergeMerchantDeltas(curMerchants, prevMerchants, maxCompareMerchants),
	}, nil
}

const maxCompareMerchants = 20

// previousRange is the period of equal length that ends the day before q.
func previousRange(q domain.RangeQuery) domain.RangeQuery {
	length := daysBetween(q.From, q.To)
	return domain.RangeQuery{
		From:             q.From.AddDate(0, 0, -length),
		To:               q.From.AddDate(0, 0, -1),
		IncludeTransfers: q.IncludeTransfers,
	}
}

func categoryKey(id *int64) int64 {
	if id == nil {
		return -1
	}
	return *id
}

func mergeCategoryDeltas(cur, prev []domain.CategoryTotal) []domain.CategoryDelta {
	byKey := make(map[int64]*domain.CategoryDelta)
	order := make([]int64, 0, len(cur)+len(prev))
	get := func(ct domain.CategoryTotal) *domain.CategoryDelta {
		k := categoryKey(ct.CategoryID)
		d, ok := byKey[k]
		if !ok {
			d = &domain.CategoryDelta{CategoryID: ct.CategoryID, Name: ct.Name, Color: ct.Color, IsTransfer: ct.IsTransfer}
			byKey[k] = d
			order = append(order, k)
		}
		return d
	}
	for _, ct := range cur {
		get(ct).Current = ct.Expense
	}
	for _, ct := range prev {
		get(ct).Previous = ct.Expense
	}
	out := make([]domain.CategoryDelta, 0, len(order))
	for _, k := range order {
		d := byKey[k]
		if d.Current == 0 && d.Previous == 0 {
			continue
		}
		out = append(out, *d)
	}
	sort.SliceStable(out, func(i, j int) bool {
		di, dj := absMoney(out[i].Current-out[i].Previous), absMoney(out[j].Current-out[j].Previous)
		if di != dj {
			return di > dj
		}
		return out[i].Current > out[j].Current
	})
	return out
}

func mergeMerchantDeltas(cur, prev []domain.MerchantTotal, limit int) []domain.MerchantDelta {
	byName := make(map[string]*domain.MerchantDelta)
	order := make([]string, 0, len(cur)+len(prev))
	get := func(name string) *domain.MerchantDelta {
		d, ok := byName[name]
		if !ok {
			d = &domain.MerchantDelta{Merchant: name}
			byName[name] = d
			order = append(order, name)
		}
		return d
	}
	for _, m := range cur {
		get(m.Merchant).Current = m.Expense
	}
	for _, m := range prev {
		get(m.Merchant).Previous = m.Expense
	}
	out := make([]domain.MerchantDelta, 0, len(order))
	for _, name := range order {
		out = append(out, *byName[name])
	}
	sort.SliceStable(out, func(i, j int) bool {
		di, dj := absMoney(out[i].Current-out[i].Previous), absMoney(out[j].Current-out[j].Previous)
		if di != dj {
			return di > dj
		}
		return out[i].Current > out[j].Current
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func absMoney(m domain.Money) domain.Money {
	if m < 0 {
		return -m
	}
	return m
}

// DataRange reports the span of dates that have data.
func (s *Service) DataRange(ctx context.Context) (domain.DateRange, error) {
	dr, err := s.analytics.DataRange(ctx)
	if err != nil {
		return domain.DateRange{}, fmt.Errorf("app: data range: %w", err)
	}
	return dr, nil
}

func validateRange(q domain.RangeQuery) error {
	if q.From.IsZero() || q.To.IsZero() {
		return fmt.Errorf("%w: from and to are required", domain.ErrInvalid)
	}
	if q.From.After(q.To) {
		return fmt.Errorf("%w: from is after to", domain.ErrInvalid)
	}
	if daysBetween(q.From, q.To) > maxRangeDays {
		return fmt.Errorf("%w: range is longer than %d days", domain.ErrInvalid, maxRangeDays)
	}
	return nil
}

// daysBetween counts inclusive calendar days between two dates.
func daysBetween(from, to time.Time) int {
	from = domain.DateOf(from)
	to = domain.DateOf(to)
	return int(to.Sub(from).Hours()/24) + 1
}

// truncateTo returns the start of the bucket containing d. Weeks start on
// Monday to match ISO 8601 and the Postgres date_trunc used by the store.
func truncateTo(d time.Time, g domain.Granularity) time.Time {
	y, m, day := d.Date()
	switch g {
	case domain.GranularityWeek:
		offset := (int(d.Weekday()) + 6) % 7
		return domain.Date(y, m, day-offset)
	case domain.GranularityMonth:
		return domain.Date(y, m, 1)
	default:
		return domain.Date(y, m, day)
	}
}

func step(d time.Time, g domain.Granularity) time.Time {
	switch g {
	case domain.GranularityWeek:
		return d.AddDate(0, 0, 7)
	case domain.GranularityMonth:
		return d.AddDate(0, 1, 0)
	default:
		return d.AddDate(0, 0, 1)
	}
}

// fillGaps produces one bucket per period between from and to, taking
// values from raw where present and zeros elsewhere.
func fillGaps(raw []domain.Bucket, from, to time.Time, g domain.Granularity) []domain.Bucket {
	byStart := make(map[time.Time]domain.Bucket, len(raw))
	for _, b := range raw {
		byStart[domain.DateOf(b.Start)] = b
	}
	out := make([]domain.Bucket, 0)
	end := truncateTo(to, g)
	for cur := truncateTo(from, g); !cur.After(end); cur = step(cur, g) {
		b, ok := byStart[cur]
		if !ok {
			b = domain.Bucket{}
		}
		b.Start = cur
		out = append(out, b)
	}
	return out
}
