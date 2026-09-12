package app

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/ALexfonSchneider/expenses/internal/domain"
)

const (
	overviewRecent       = 10
	overviewTrendMonths  = 6
	typicalWeeksLookback = 8
	staleStatementAfter  = 7 * 24 * time.Hour
	staleSyncAfter       = 3 * 24 * time.Hour
	largeSpendFloor      = domain.Money(500000)
	largeSpendFactor     = 3
	largeSpendWindow     = 7
	largeSpendBaseline   = 90
	maxLargeSpendSignals = 3
)

// Overview assembles the home page for today.
func (s *Service) Overview(ctx context.Context) (*domain.Overview, error) {
	today := domain.DateOf(s.now())
	ov := &domain.Overview{Today: today}

	month, err := s.monthOverview(ctx, today)
	if err != nil {
		return nil, err
	}
	ov.Month = month

	week, err := s.weekOverview(ctx, today)
	if err != nil {
		return nil, err
	}
	ov.Week = week

	trend, err := s.trendOverview(ctx, today)
	if err != nil {
		return nil, err
	}
	ov.Trend = trend

	recent, _, err := s.transactions.List(ctx, domain.TransactionFilter{IncludeTransfers: true, Limit: overviewRecent})
	if err != nil {
		return nil, fmt.Errorf("app: recent transactions: %w", err)
	}
	ov.Recent = recent

	signals, err := s.signals(ctx, today, month)
	if err != nil {
		return nil, err
	}
	ov.Signals = signals
	return ov, nil
}

func (s *Service) monthOverview(ctx context.Context, today time.Time) (domain.MonthOverview, error) {
	report, err := s.BudgetReport(ctx, today)
	if err != nil {
		return domain.MonthOverview{}, err
	}
	prevStart := report.Month.AddDate(0, -1, 0)
	prevEnd := report.Month.AddDate(0, 0, -1)
	prevPoint := prevStart.AddDate(0, 0, report.DaysElapsed-1)
	if prevPoint.After(prevEnd) {
		prevPoint = prevEnd
	}
	samePoint, err := s.analytics.Totals(ctx, domain.RangeQuery{From: prevStart, To: prevPoint})
	if err != nil {
		return domain.MonthOverview{}, fmt.Errorf("app: previous month to date: %w", err)
	}
	prevTotal, err := s.analytics.Totals(ctx, domain.RangeQuery{From: prevStart, To: prevEnd})
	if err != nil {
		return domain.MonthOverview{}, fmt.Errorf("app: previous month: %w", err)
	}
	return domain.MonthOverview{
		Month:         report.Month,
		DaysElapsed:   report.DaysElapsed,
		DaysInMonth:   report.DaysInMonth,
		Spent:         report.Total.Spent,
		Forecast:      report.Total.Forecast,
		Limit:         report.Total.Limit,
		PaceLimit:     report.Total.PaceLimit,
		PrevSamePoint: samePoint.Expense,
		PrevTotal:     prevTotal.Expense,
	}, nil
}

func (s *Service) weekOverview(ctx context.Context, today time.Time) (domain.WeekOverview, error) {
	from := today.AddDate(0, 0, -6)
	days, err := s.Series(ctx, domain.SeriesQuery{RangeQuery: domain.RangeQuery{From: from, To: today}, Granularity: domain.GranularityDay})
	if err != nil {
		return domain.WeekOverview{}, err
	}
	var spent domain.Money
	for _, d := range days {
		spent += d.Expense
	}

	// Typical week: the completed ISO weeks before the current one.
	thisMonday := truncateTo(today, domain.GranularityWeek)
	baseTo := thisMonday.AddDate(0, 0, -1)
	baseFrom := thisMonday.AddDate(0, 0, -7*typicalWeeksLookback)
	weeks, err := s.analytics.Series(ctx, domain.SeriesQuery{RangeQuery: domain.RangeQuery{From: baseFrom, To: baseTo}, Granularity: domain.GranularityWeek})
	if err != nil {
		return domain.WeekOverview{}, fmt.Errorf("app: typical week: %w", err)
	}
	var typical domain.Money
	n := 0
	for _, w := range weeks {
		if w.Expense > 0 {
			typical += w.Expense
			n++
		}
	}
	if n > 0 {
		typical /= domain.Money(n)
	}
	return domain.WeekOverview{From: from, To: today, Spent: spent, Days: days, TypicalWeek: typical, WeeksInAvg: n}, nil
}

func (s *Service) trendOverview(ctx context.Context, today time.Time) (domain.TrendOverview, error) {
	start := truncateTo(today, domain.GranularityMonth).AddDate(0, -(overviewTrendMonths - 1), 0)
	months, err := s.Series(ctx, domain.SeriesQuery{RangeQuery: domain.RangeQuery{From: start, To: today}, Granularity: domain.GranularityMonth})
	if err != nil {
		return domain.TrendOverview{}, err
	}
	// The current month is incomplete and would drag the average down.
	var sum domain.Money
	n := 0
	for i, m := range months {
		if i == len(months)-1 || m.Expense == 0 {
			continue
		}
		sum += m.Expense
		n++
	}
	var avg domain.Money
	if n > 0 {
		avg = sum / domain.Money(n)
	}
	return domain.TrendOverview{Months: months, Average: avg}, nil
}

// signals lists what deserves attention, worst first.
func (s *Service) signals(ctx context.Context, today time.Time, month domain.MonthOverview) ([]domain.Signal, error) {
	out := make([]domain.Signal, 0)

	report, err := s.BudgetReport(ctx, today)
	if err != nil {
		return nil, err
	}
	budgets := append([]domain.BudgetStatus{}, report.Items...)
	if report.Total.Limit > 0 {
		budgets = append(budgets, report.Total)
	}
	for _, b := range budgets {
		switch {
		case b.Spent >= b.Limit:
			out = append(out, domain.Signal{
				Kind: "budget", Severity: domain.SeverityBad,
				Title:  fmt.Sprintf("Бюджет «%s» превышен на %s", b.Name, formatRubles(b.Spent-b.Limit)),
				Detail: fmt.Sprintf("потрачено %s из %s", formatRubles(b.Spent), formatRubles(b.Limit)),
				Link:   "/dashboard?preset=month",
			})
		case b.Spent > b.PaceLimit && month.DaysElapsed < month.DaysInMonth:
			out = append(out, domain.Signal{
				Kind: "budget", Severity: domain.SeverityWarn,
				Title:  fmt.Sprintf("«%s» идёт быстрее плана", b.Name),
				Detail: fmt.Sprintf("%s из %s за %d из %d дней, прогноз %s", formatRubles(b.Spent), formatRubles(b.Limit), month.DaysElapsed, month.DaysInMonth, formatRubles(b.Forecast)),
				Link:   "/dashboard?preset=month",
			})
		}
	}

	large, err := s.largeSpends(ctx, today)
	if err != nil {
		return nil, err
	}
	out = append(out, large...)

	uncategorizedFrom := today.AddDate(0, 0, -largeSpendBaseline)
	_, uncategorized, err := s.transactions.List(ctx, domain.TransactionFilter{From: &uncategorizedFrom, To: &today, Uncategorized: true, Limit: 1})
	if err != nil {
		return nil, fmt.Errorf("app: uncategorized count: %w", err)
	}
	if uncategorized > 0 {
		out = append(out, domain.Signal{
			Kind: "uncategorized", Severity: domain.SeverityInfo,
			Title:  fmt.Sprintf("Без категории: %d операций за 90 дней", uncategorized),
			Detail: "правило по продавцу закрывает всех будущих",
			Link:   "/transactions?preset=90d&uncategorized=1",
		})
	}

	statements, err := s.statements.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("app: list statements: %w", err)
	}
	if len(statements) == 0 {
		out = append(out, domain.Signal{Kind: "statement", Severity: domain.SeverityWarn, Title: "Нет ни одной выписки", Detail: "загрузите PDF из банка", Link: "/statements"})
	} else {
		latest := statements[0].PeriodTo
		for _, st := range statements {
			if st.PeriodTo.After(latest) {
				latest = st.PeriodTo
			}
		}
		if gap := today.Sub(latest); gap > staleStatementAfter {
			out = append(out, domain.Signal{
				Kind: "statement", Severity: domain.SeverityWarn,
				Title:  fmt.Sprintf("Выписка заканчивается %s", latest.Format("02.01.2006")),
				Detail: fmt.Sprintf("%d дней без данных банка, траты видны только по чекам", int(gap.Hours()/24)),
				Link:   "/statements",
			})
		}
	}

	session, err := s.sessions.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("app: load receipt session: %w", err)
	}
	if session == nil {
		out = append(out, domain.Signal{Kind: "receipts", Severity: domain.SeverityInfo, Title: "Архив чеков не подключён", Detail: "чеки дают состав покупок и траты с других карт", Link: "/receipts"})
	} else {
		status, err := s.ReceiptSyncStatus(ctx)
		if err != nil {
			return nil, err
		}
		switch {
		case status.Error != "" && !status.Running:
			severity := domain.SeverityWarn
			title := "Синхронизация чеков завершилась ошибкой"
			if errors.Is(errors.New(status.Error), domain.ErrReceiptSession) || containsSessionError(status.Error) {
				title = "Сессия архива чеков истекла"
			}
			out = append(out, domain.Signal{Kind: "receipts", Severity: severity, Title: title, Detail: status.Error, Link: "/receipts"})
		case !status.Running && !status.FinishedAt.IsZero() && today.Sub(domain.DateOf(status.FinishedAt)) > staleSyncAfter:
			out = append(out, domain.Signal{Kind: "receipts", Severity: domain.SeverityInfo, Title: "Чеки давно не синхронизировались", Detail: "последний раз " + status.FinishedAt.Format("02.01.2006"), Link: "/receipts"})
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		return severityRank(out[i].Severity) < severityRank(out[j].Severity)
	})
	return out, nil
}

// largeSpends flags the biggest recent operations: three times the average
// expense of the last three months, but never below a fixed floor, so a
// normal week does not raise alarms.
func (s *Service) largeSpends(ctx context.Context, today time.Time) ([]domain.Signal, error) {
	baseFrom := today.AddDate(0, 0, -largeSpendBaseline)
	base, err := s.analytics.Totals(ctx, domain.RangeQuery{From: baseFrom, To: today})
	if err != nil {
		return nil, fmt.Errorf("app: baseline totals: %w", err)
	}
	threshold := largeSpendFloor
	if base.Count > 0 {
		if avg := base.Expense / domain.Money(base.Count) * largeSpendFactor; avg > threshold {
			threshold = avg
		}
	}
	from := today.AddDate(0, 0, -largeSpendWindow)
	recent, _, err := s.transactions.List(ctx, domain.TransactionFilter{From: &from, To: &today, Direction: domain.DirectionExpense, Limit: maxPageSize})
	if err != nil {
		return nil, fmt.Errorf("app: recent expenses: %w", err)
	}
	out := make([]domain.Signal, 0)
	for _, t := range recent {
		if t.Excluded || t.Amount < threshold {
			continue
		}
		name := t.Merchant
		if name == "" {
			name = t.Description
		}
		out = append(out, domain.Signal{
			Kind: "large", Severity: domain.SeverityInfo,
			Title:  fmt.Sprintf("Крупная трата: %s, %s", formatRubles(t.Amount), name),
			Detail: t.OpAt.Format("02.01.2006") + ", разовую покупку можно исключить из статистики",
			Link:   fmt.Sprintf("/transactions?preset=custom&from=%s&to=%s&id=%d", domain.DateOf(t.OpAt).AddDate(0, 0, -1).Format("2006-01-02"), domain.DateOf(t.OpAt).AddDate(0, 0, 1).Format("2006-01-02"), t.ID),
		})
		if len(out) == maxLargeSpendSignals {
			break
		}
	}
	return out, nil
}

func severityRank(s domain.SignalSeverity) int {
	switch s {
	case domain.SeverityBad:
		return 0
	case domain.SeverityWarn:
		return 1
	default:
		return 2
	}
}

func containsSessionError(msg string) bool {
	return len(msg) > 0 && (contains(msg, "session") || contains(msg, "refresh token"))
}

func contains(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// formatRubles renders kopecks as whole rubles for signal text.
func formatRubles(m domain.Money) string {
	sign := ""
	if m < 0 {
		sign = "-"
		m = -m
	}
	rub := int64(m / 100)
	digits := fmt.Sprintf("%d", rub)
	var b []byte
	for i, c := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b = append(b, ' ')
		}
		b = append(b, byte(c))
	}
	return sign + string(b) + " ₽"
}
