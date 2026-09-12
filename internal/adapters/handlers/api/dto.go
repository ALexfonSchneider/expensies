package api

import (
	"time"

	"github.com/ALexfonSchneider/expenses/internal/domain"
)

// Amounts are integers in kopecks so the frontend formats without float
// drift. Dates are YYYY-MM-DD; timestamps are RFC 3339 in the statement zone.

type listDTO[T any] struct {
	Items []T `json:"items"`
	Total int `json:"total"`
}

type statementDTO struct {
	ID             int64  `json:"id"`
	Bank           string `json:"bank"`
	Number         string `json:"number"`
	PeriodFrom     string `json:"period_from"`
	PeriodTo       string `json:"period_to"`
	OpeningBalance int64  `json:"opening_balance"`
	ClosingBalance int64  `json:"closing_balance"`
	TotalIncome    int64  `json:"total_income"`
	TotalExpense   int64  `json:"total_expense"`
	FileName       string `json:"file_name"`
	TxCount        int    `json:"tx_count"`
	UploadedAt     string `json:"uploaded_at"`
}

type importResultDTO struct {
	Statement  statementDTO `json:"statement"`
	Imported   int          `json:"imported"`
	Duplicates int          `json:"duplicates"`
	Warnings   []string     `json:"warnings"`
}

type transactionDTO struct {
	ID             int64  `json:"id"`
	StatementID    int64  `json:"statement_id"`
	Source         string `json:"source"`
	OpAt           string `json:"op_at"`
	OpDate         string `json:"op_date"`
	ProcessedOn    string `json:"processed_on"`
	Card           string `json:"card"`
	Amount         int64  `json:"amount"`
	Direction      string `json:"direction"`
	Kind           string `json:"kind"`
	Description    string `json:"description"`
	Merchant       string `json:"merchant"`
	MCC            string `json:"mcc"`
	Counterparty   string `json:"counterparty"`
	CategoryID     *int64 `json:"category_id"`
	CategorySource string `json:"category_source"`
	Note           string `json:"note"`
	Excluded       bool   `json:"excluded"`
	ReceiptID      *int64 `json:"receipt_id"`
}

// transactionListDTO carries the sums of the whole filter next to the page.
type transactionListDTO struct {
	Items   []transactionDTO `json:"items"`
	Total   int              `json:"total"`
	Expense int64            `json:"expense"`
	Income  int64            `json:"income"`
}

type categoryDTO struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Color      string `json:"color"`
	IsTransfer bool   `json:"is_transfer"`
	SortOrder  int    `json:"sort_order"`
}

type ruleDTO struct {
	ID         int64  `json:"id"`
	CategoryID int64  `json:"category_id"`
	Field      string `json:"field"`
	Op         string `json:"op"`
	Value      string `json:"value"`
	Priority   int    `json:"priority"`
}

type bucketDTO struct {
	Start   string `json:"start"`
	Expense int64  `json:"expense"`
	Income  int64  `json:"income"`
	Count   int    `json:"count"`
}

type totalsDTO struct {
	Expense int64 `json:"expense"`
	Income  int64 `json:"income"`
	Count   int   `json:"count"`
}

type summaryDTO struct {
	From            string     `json:"from"`
	To              string     `json:"to"`
	Expense         int64      `json:"expense"`
	Income          int64      `json:"income"`
	Count           int        `json:"count"`
	Days            int        `json:"days"`
	ActiveDays      int        `json:"active_days"`
	AvgPerDay       int64      `json:"avg_per_day"`
	AvgPerActiveDay int64      `json:"avg_per_active_day"`
	MaxDay          *bucketDTO `json:"max_day"`
	Prev            totalsDTO  `json:"prev"`
}

type seriesDTO struct {
	Granularity string      `json:"granularity"`
	Items       []bucketDTO `json:"items"`
}

type categoryTotalDTO struct {
	CategoryID *int64 `json:"category_id"`
	Name       string `json:"name"`
	Color      string `json:"color"`
	IsTransfer bool   `json:"is_transfer"`
	Expense    int64  `json:"expense"`
	Income     int64  `json:"income"`
	Count      int    `json:"count"`
}

type merchantTotalDTO struct {
	Merchant string `json:"merchant"`
	Expense  int64  `json:"expense"`
	Count    int    `json:"count"`
}

type rangeDTO struct {
	From    string `json:"from"`
	To      string `json:"to"`
	HasData bool   `json:"has_data"`
}

func fmtDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(dateLayout)
}

func (h *Handler) fmtTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.In(h.loc).Format(time.RFC3339)
}

func (h *Handler) statementDTO(st domain.Statement) statementDTO {
	return statementDTO{
		ID:             st.ID,
		Bank:           st.Bank,
		Number:         st.Number,
		PeriodFrom:     fmtDate(st.PeriodFrom),
		PeriodTo:       fmtDate(st.PeriodTo),
		OpeningBalance: int64(st.OpeningBalance),
		ClosingBalance: int64(st.ClosingBalance),
		TotalIncome:    int64(st.TotalIncome),
		TotalExpense:   int64(st.TotalExpense),
		FileName:       st.FileName,
		TxCount:        st.TxCount,
		UploadedAt:     h.fmtTime(st.UploadedAt),
	}
}

func (h *Handler) importResultDTO(res *domain.ImportResult) importResultDTO {
	warnings := res.Warnings
	if warnings == nil {
		warnings = []string{}
	}
	return importResultDTO{
		Statement:  h.statementDTO(res.Statement),
		Imported:   res.Imported,
		Duplicates: res.Duplicates,
		Warnings:   warnings,
	}
}

func (h *Handler) transactionDTO(t domain.Transaction) transactionDTO {
	return transactionDTO{
		ID:             t.ID,
		StatementID:    t.StatementID,
		Source:         string(t.Source),
		OpAt:           h.fmtTime(t.OpAt),
		OpDate:         fmtDate(domain.DateOf(t.OpAt)),
		ProcessedOn:    fmtDate(t.ProcessedOn),
		Card:           t.Card,
		Amount:         int64(t.Amount),
		Direction:      string(t.Direction),
		Kind:           string(t.Kind),
		Description:    t.Description,
		Merchant:       t.Merchant,
		MCC:            t.MCC,
		Counterparty:   t.Counterparty,
		CategoryID:     t.CategoryID,
		CategorySource: string(t.CategorySource),
		Note:           t.Note,
		Excluded:       t.Excluded,
		ReceiptID:      t.ReceiptID,
	}
}

type budgetDTO struct {
	ID         int64  `json:"id"`
	CategoryID *int64 `json:"category_id"`
	Amount     int64  `json:"amount"`
}

type budgetStatusDTO struct {
	BudgetID   int64  `json:"budget_id"`
	CategoryID *int64 `json:"category_id"`
	Name       string `json:"name"`
	Color      string `json:"color"`
	Limit      int64  `json:"limit"`
	Spent      int64  `json:"spent"`
	Forecast   int64  `json:"forecast"`
	PaceLimit  int64  `json:"pace_limit"`
}

type budgetReportDTO struct {
	Month       string            `json:"month"`
	DaysInMonth int               `json:"days_in_month"`
	DaysElapsed int               `json:"days_elapsed"`
	Items       []budgetStatusDTO `json:"items"`
	Total       budgetStatusDTO   `json:"total"`
}

type periodDTO struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type categoryDeltaDTO struct {
	CategoryID *int64 `json:"category_id"`
	Name       string `json:"name"`
	Color      string `json:"color"`
	IsTransfer bool   `json:"is_transfer"`
	Current    int64  `json:"current"`
	Previous   int64  `json:"previous"`
}

type merchantDeltaDTO struct {
	Merchant string `json:"merchant"`
	Current  int64  `json:"current"`
	Previous int64  `json:"previous"`
}

type comparisonDTO struct {
	Current        periodDTO          `json:"current"`
	Previous       periodDTO          `json:"previous"`
	CurrentTotals  totalsDTO          `json:"current_totals"`
	PreviousTotals totalsDTO          `json:"previous_totals"`
	Categories     []categoryDeltaDTO `json:"categories"`
	Merchants      []merchantDeltaDTO `json:"merchants"`
}

func budgetDTOOf(b domain.Budget) budgetDTO {
	return budgetDTO{ID: b.ID, CategoryID: b.CategoryID, Amount: int64(b.Amount)}
}

func budgetStatusDTOOf(s domain.BudgetStatus) budgetStatusDTO {
	return budgetStatusDTO{
		BudgetID:   s.BudgetID,
		CategoryID: s.CategoryID,
		Name:       s.Name,
		Color:      s.Color,
		Limit:      int64(s.Limit),
		Spent:      int64(s.Spent),
		Forecast:   int64(s.Forecast),
		PaceLimit:  int64(s.PaceLimit),
	}
}

func budgetReportDTOOf(r *domain.BudgetReport) budgetReportDTO {
	items := make([]budgetStatusDTO, 0, len(r.Items))
	for _, s := range r.Items {
		items = append(items, budgetStatusDTOOf(s))
	}
	return budgetReportDTO{
		Month:       r.Month.Format(monthLayout),
		DaysInMonth: r.DaysInMonth,
		DaysElapsed: r.DaysElapsed,
		Items:       items,
		Total:       budgetStatusDTOOf(r.Total),
	}
}

func totalsDTOOf(t domain.Totals) totalsDTO {
	return totalsDTO{Expense: int64(t.Expense), Income: int64(t.Income), Count: t.Count}
}

func comparisonDTOOf(c *domain.Comparison) comparisonDTO {
	cats := make([]categoryDeltaDTO, 0, len(c.Categories))
	for _, d := range c.Categories {
		cats = append(cats, categoryDeltaDTO{
			CategoryID: d.CategoryID,
			Name:       d.Name,
			Color:      d.Color,
			IsTransfer: d.IsTransfer,
			Current:    int64(d.Current),
			Previous:   int64(d.Previous),
		})
	}
	merchants := make([]merchantDeltaDTO, 0, len(c.Merchants))
	for _, d := range c.Merchants {
		merchants = append(merchants, merchantDeltaDTO{Merchant: d.Merchant, Current: int64(d.Current), Previous: int64(d.Previous)})
	}
	return comparisonDTO{
		Current:        periodDTO{From: fmtDate(c.Current.From), To: fmtDate(c.Current.To)},
		Previous:       periodDTO{From: fmtDate(c.Previous.From), To: fmtDate(c.Previous.To)},
		CurrentTotals:  totalsDTOOf(c.CurrentTotals),
		PreviousTotals: totalsDTOOf(c.PreviousTotals),
		Categories:     cats,
		Merchants:      merchants,
	}
}

func categoryDTOOf(c domain.Category) categoryDTO {
	return categoryDTO{ID: c.ID, Name: c.Name, Color: c.Color, IsTransfer: c.IsTransfer, SortOrder: c.SortOrder}
}

func ruleDTOOf(r domain.Rule) ruleDTO {
	return ruleDTO{ID: r.ID, CategoryID: r.CategoryID, Field: string(r.Field), Op: string(r.Op), Value: r.Value, Priority: r.Priority}
}

func bucketDTOOf(b domain.Bucket) bucketDTO {
	return bucketDTO{Start: fmtDate(b.Start), Expense: int64(b.Expense), Income: int64(b.Income), Count: b.Count}
}

func summaryDTOOf(s *domain.Summary) summaryDTO {
	out := summaryDTO{
		From:            fmtDate(s.From),
		To:              fmtDate(s.To),
		Expense:         int64(s.Expense),
		Income:          int64(s.Income),
		Count:           s.Count,
		Days:            s.Days,
		ActiveDays:      s.ActiveDays,
		AvgPerDay:       int64(s.AvgPerDay),
		AvgPerActiveDay: int64(s.AvgPerActiveDay),
		Prev:            totalsDTO{Expense: int64(s.Prev.Expense), Income: int64(s.Prev.Income), Count: s.Prev.Count},
	}
	if s.MaxDay != nil {
		b := bucketDTOOf(*s.MaxDay)
		out.MaxDay = &b
	}
	return out
}
