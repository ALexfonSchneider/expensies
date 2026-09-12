package yandexpdf

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ALexfonSchneider/expenses/internal/domain"
)

// ErrUnsupported is returned when the file is not a Yandex Bank statement
// or its layout differs from the one this parser understands.
var ErrUnsupported = errors.New("unsupported statement format")

// item is one positioned text run on a page.
type item struct {
	x    float64
	text string
}

// row is a group of items sharing a baseline.
type row struct {
	y     int64
	items []item
}

func (r row) text() string {
	parts := make([]string, 0, len(r.items))
	for _, it := range r.items {
		parts = append(parts, it.text)
	}
	return strings.Join(parts, " ")
}

func (r row) first() string {
	if len(r.items) == 0 {
		return ""
	}
	return r.items[0].text
}

func (r row) last() string {
	if len(r.items) == 0 {
		return ""
	}
	return r.items[len(r.items)-1].text
}

func (r row) find(col column, re *regexp.Regexp) (string, bool) {
	for _, it := range r.items {
		if columnOf(it.x) == col && re.MatchString(it.text) {
			return it.text, true
		}
	}
	return "", false
}

func (r row) has(text string) bool {
	for _, it := range r.items {
		if it.text == text {
			return true
		}
	}
	return false
}

type column int

const (
	colDesc column = iota
	colOpDate
	colProcDate
	colCard
	colAmount
	colAmountAcc
)

// colBounds are midpoints between the header cell origins of the real
// layout (20, 210, 312, 358, 415, 510). validateHeader checks every page
// against them so a changed layout fails loudly instead of silently
// mis-assigning columns.
var colBounds = [...]float64{115, 261, 335, 386, 462}

func columnOf(x float64) column {
	for i, b := range colBounds {
		if x < b {
			return column(i)
		}
	}
	return colAmountAcc
}

var (
	reDate    = regexp.MustCompile(`^(\d{2})\.(\d{2})\.(\d{4})$`)
	reTime    = regexp.MustCompile(`^в (\d{2}):(\d{2})$`)
	reCard    = regexp.MustCompile(`^\*\d{4}$`)
	reAmount  = regexp.MustCompile(`^([+\-\x{2013}\x{2212}])?\s*([\d\s\x{00a0}]+),(\d{2})\s*(?:₽|RUB|руб\.?)?$`)
	reNumber  = regexp.MustCompile(`Исх\.\s*№\s*(\d+)`)
	rePeriod  = regexp.MustCompile(`за период с (\d{2}\.\d{2}\.\d{4}) по (\d{2}\.\d{2}\.\d{4})`)
	rePage    = regexp.MustCompile(`^Страница \d+ из \d+$`)
	reOpening = regexp.MustCompile(`^Входящий остаток на \d{2}\.\d{2}\.\d{4}`)
	reClosing = regexp.MustCompile(`^Исходящий остаток за \d{2}\.\d{2}\.\d{4}`)
	reSpaces  = regexp.MustCompile(`\s+`)
)

const (
	labelHeaderDesc   = "Описание операции"
	labelHeaderOpDate = "Дата и время"
	labelHeaderProc   = "Дата"
	labelHeaderCard   = "Карта"
	labelHeaderAmount = "Сумма в валюте"
	labelContinue     = "Продолжение на следующей странице"
	labelTotalExpense = "Всего расходных операций"
	labelTotalIncome  = "Всего приходных операций"
	labelBank         = "Яндекс Банк"
)

// pending accumulates the rows of one transaction until the next starts.
type pending struct {
	page      int
	descLines []string
	opDate    string
	procDate  string
	timeStr   string
	card      string
	amountOp  string
	amountAcc string
}

func (p *pending) absorb(r row) {
	for _, it := range r.items {
		switch columnOf(it.x) {
		case colDesc:
			p.descLines = append(p.descLines, it.text)
		case colOpDate:
			if reTime.MatchString(it.text) {
				p.timeStr = it.text
			}
		case colCard:
			if reCard.MatchString(it.text) {
				p.card = it.text
			}
		}
	}
}

type builder struct {
	ps         *domain.ParsedStatement
	loc        *time.Location
	seq        map[string]int
	cur        *pending
	sumExpense domain.Money
	sumIncome  domain.Money
}

func (b *builder) warnf(format string, args ...any) {
	b.ps.Warnings = append(b.ps.Warnings, fmt.Sprintf(format, args...))
}

func (b *builder) start(r row, page int) {
	p := &pending{page: page}
	p.opDate, _ = r.find(colOpDate, reDate)
	p.procDate, _ = r.find(colProcDate, reDate)
	p.amountOp, _ = r.find(colAmount, reAmount)
	p.amountAcc, _ = r.find(colAmountAcc, reAmount)
	p.absorb(r)
	b.cur = p
}

func (b *builder) flush() {
	p := b.cur
	if p == nil {
		return
	}
	b.cur = nil

	tx, err := b.transaction(p)
	if err != nil {
		b.warnf("page %d: skipped operation %q: %v", p.page, joinLines(p.descLines), err)
		return
	}

	key := fmt.Sprintf("%s|%s|%d|%s", tx.OpAt.Format(time.RFC3339), tx.Direction, tx.Amount, tx.Description)
	seq := b.seq[key]
	b.seq[key] = seq + 1
	tx.Fingerprint = domain.Fingerprint(Bank, tx.OpAt, tx.Direction, tx.Amount, tx.Description, seq)

	switch tx.Direction {
	case domain.DirectionExpense:
		b.sumExpense += tx.Amount
	case domain.DirectionIncome:
		b.sumIncome += tx.Amount
	}
	b.ps.Transactions = append(b.ps.Transactions, tx)
}

func (b *builder) transaction(p *pending) (domain.Transaction, error) {
	desc := joinLines(p.descLines)

	opDate, err := parseDate(p.opDate)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("operation date: %w", err)
	}
	hour, minute := 0, 0
	if m := reTime.FindStringSubmatch(p.timeStr); m != nil {
		hour, minute = atoi(m[1]), atoi(m[2])
	} else {
		b.warnf("page %d: operation %q has no time, using midnight", p.page, desc)
	}
	y, mo, d := opDate.Date()
	opAt := time.Date(y, mo, d, hour, minute, 0, 0, b.loc)

	processed := opDate
	if p.procDate != "" {
		processed, err = parseDate(p.procDate)
		if err != nil {
			return domain.Transaction{}, fmt.Errorf("processing date: %w", err)
		}
	}

	amountText := p.amountAcc
	if amountText == "" {
		amountText = p.amountOp
	}
	amount, sign, err := parseAmount(amountText)
	if err != nil {
		return domain.Transaction{}, fmt.Errorf("amount: %w", err)
	}
	if amount == 0 {
		return domain.Transaction{}, fmt.Errorf("amount is zero")
	}
	if p.amountOp != "" && p.amountAcc != "" && digitsOf(p.amountOp) != digitsOf(p.amountAcc) {
		b.warnf("page %d: operation %q: amount in operation currency %q differs from account currency %q",
			p.page, desc, p.amountOp, p.amountAcc)
	}

	dir := domain.DirectionExpense
	switch {
	case sign > 0:
		dir = domain.DirectionIncome
	case sign == 0:
		b.warnf("page %d: operation %q has no sign, treating as expense", p.page, desc)
	}

	kind, merchant, mcc, counterparty := classify(desc)

	return domain.Transaction{
		OpAt:         opAt,
		ProcessedOn:  processed,
		Card:         p.card,
		Amount:       amount,
		Direction:    dir,
		Kind:         kind,
		Description:  desc,
		Merchant:     merchant,
		MCC:          mcc,
		Counterparty: counterparty,
	}, nil
}

// metaRow consumes statement-level rows (number, period, balances, totals)
// that live outside the operations table. Returns true when consumed.
func (b *builder) metaRow(r row) bool {
	st := &b.ps.Statement
	line := r.text()
	first := r.first()
	switch {
	case reNumber.MatchString(line):
		if st.Number == "" {
			st.Number = reNumber.FindStringSubmatch(line)[1]
		}
	case rePeriod.MatchString(line):
		m := rePeriod.FindStringSubmatch(line)
		from, errFrom := parseDate(m[1])
		to, errTo := parseDate(m[2])
		if errFrom != nil || errTo != nil {
			b.warnf("statement period %q is not parseable", line)
			return true
		}
		st.PeriodFrom, st.PeriodTo = from, to
	case reOpening.MatchString(first):
		st.OpeningBalance = b.trailingAmount(r, "opening balance")
	case reClosing.MatchString(first):
		st.ClosingBalance = b.trailingAmount(r, "closing balance")
	case first == labelTotalExpense:
		st.TotalExpense = abs(b.trailingAmount(r, "total expenses"))
	case first == labelTotalIncome:
		st.TotalIncome = abs(b.trailingAmount(r, "total income"))
	default:
		return false
	}
	return true
}

// trailingAmount parses the right-most cell of a row as a signed amount.
func (b *builder) trailingAmount(r row, what string) domain.Money {
	amount, sign, err := parseAmount(r.last())
	if err != nil {
		b.warnf("%s: %v", what, err)
		return 0
	}
	if sign < 0 {
		return -amount
	}
	return amount
}

func (b *builder) verifyTotals() {
	st := b.ps.Statement
	if len(b.ps.Transactions) == 0 {
		b.warnf("no operations found in the statement")
		return
	}
	if st.TotalExpense == 0 && st.TotalIncome == 0 {
		b.warnf("statement totals not found; parsed sums are unverified")
		return
	}
	if st.TotalExpense != b.sumExpense {
		b.warnf("sum of parsed expenses %s differs from statement total %s",
			formatMoney(b.sumExpense), formatMoney(st.TotalExpense))
	}
	if st.TotalIncome != b.sumIncome {
		b.warnf("sum of parsed income %s differs from statement total %s",
			formatMoney(b.sumIncome), formatMoney(st.TotalIncome))
	}
}

// parsePages is the pure core of the parser, separated from PDF access so
// it can be tested with synthetic rows.
func parsePages(pages [][]row, loc *time.Location) (*domain.ParsedStatement, error) {
	b := &builder{
		ps:  &domain.ParsedStatement{Statement: domain.Statement{Bank: Bank}},
		loc: loc,
		seq: map[string]int{},
	}
	headerFound := false
	bankFound := false

	for pi, rows := range pages {
		pageNo := pi + 1
		inTable := false
		for _, r := range rows {
			if strings.Contains(r.text(), labelBank) {
				bankFound = true
			}
			if isFooter(r) {
				b.flush()
				inTable = false
				continue
			}
			if b.metaRow(r) {
				b.flush()
				continue
			}
			if r.has(labelHeaderDesc) {
				if err := validateHeader(r, pageNo); err != nil {
					return nil, err
				}
				b.flush()
				headerFound = true
				inTable = true
				continue
			}
			if !inTable {
				continue
			}
			if isTxStart(r) {
				b.flush()
				b.start(r, pageNo)
				continue
			}
			if b.cur != nil {
				b.cur.absorb(r)
			}
		}
		// JasperReports never splits a table row across pages, so a page
		// boundary always ends the current operation.
		b.flush()
	}

	if !headerFound {
		return nil, fmt.Errorf("%w: operations table not found", ErrUnsupported)
	}
	if !bankFound {
		return nil, fmt.Errorf("%w: issuer %q not found", ErrUnsupported, labelBank)
	}
	st := &b.ps.Statement
	if st.Number == "" {
		return nil, fmt.Errorf("%w: statement number not found", ErrUnsupported)
	}
	st.TxCount = len(b.ps.Transactions)
	b.verifyTotals()
	return b.ps, nil
}

func isFooter(r row) bool {
	for _, it := range r.items {
		if it.text == labelContinue || rePage.MatchString(it.text) {
			return true
		}
	}
	return false
}

// isTxStart recognizes the first row of an operation: a date in the
// operation-date column plus an amount on the right.
func isTxStart(r row) bool {
	if _, ok := r.find(colOpDate, reDate); !ok {
		return false
	}
	if _, ok := r.find(colAmountAcc, reAmount); ok {
		return true
	}
	_, ok := r.find(colAmount, reAmount)
	return ok
}

func validateHeader(r row, page int) error {
	expected := []struct {
		col   column
		label string
	}{
		{colDesc, labelHeaderDesc},
		{colOpDate, labelHeaderOpDate},
		{colProcDate, labelHeaderProc},
		{colCard, labelHeaderCard},
		{colAmount, labelHeaderAmount},
		{colAmountAcc, labelHeaderAmount},
	}
	for _, e := range expected {
		found := false
		for _, it := range r.items {
			if it.text == e.label && columnOf(it.x) == e.col {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("%w: page %d: header cell %q is not in the expected column", ErrUnsupported, page, e.label)
		}
	}
	return nil
}

// joinLines reassembles a wrapped cell. A trailing hyphen means the word
// continues on the next line ("Т-" + "Банк"), everything else is joined
// with a space.
func joinLines(lines []string) string {
	var b strings.Builder
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		if b.Len() > 0 && !strings.HasSuffix(b.String(), "-") {
			b.WriteByte(' ')
		}
		b.WriteString(l)
	}
	return collapseSpaces(b.String())
}

func collapseSpaces(s string) string {
	s = strings.ReplaceAll(s, " ", " ")
	return strings.TrimSpace(reSpaces.ReplaceAllString(s, " "))
}

func parseDate(s string) (time.Time, error) {
	m := reDate.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return time.Time{}, fmt.Errorf("%q is not a dd.mm.yyyy date", s)
	}
	d, mo, y := atoi(m[1]), atoi(m[2]), atoi(m[3])
	t := domain.Date(y, time.Month(mo), d)
	if t.Day() != d || int(t.Month()) != mo || t.Year() != y {
		return time.Time{}, fmt.Errorf("%q is not a valid calendar date", s)
	}
	return t, nil
}

// parseAmount reads "–1 254,00 ₽" style cells. The sign is returned
// separately (+1, -1 or 0 when absent) because balances and totals use the
// magnitude differently from operations.
func parseAmount(s string) (domain.Money, int, error) {
	m := reAmount.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return 0, 0, fmt.Errorf("%q is not an amount", s)
	}
	sign := 0
	switch m[1] {
	case "+":
		sign = 1
	case "-", "–", "−":
		sign = -1
	}
	digits := digitsOf(m[2])
	if digits == "" {
		return 0, 0, fmt.Errorf("%q has no digits", s)
	}
	whole, err := strconv.ParseInt(digits, 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("%q: %w", s, err)
	}
	return domain.Money(whole*100 + int64(atoi(m[3]))), sign, nil
}

func digitsOf(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, s)
}

// atoi converts a string that a regexp has already constrained to ASCII
// digits, so no error path is needed.
func atoi(s string) int {
	n := 0
	for _, c := range s {
		n = n*10 + int(c-'0')
	}
	return n
}

func abs(m domain.Money) domain.Money {
	if m < 0 {
		return -m
	}
	return m
}

func formatMoney(m domain.Money) string {
	sign := ""
	if m < 0 {
		sign = "-"
		m = -m
	}
	return fmt.Sprintf("%s%d.%02d", sign, m/100, m%100)
}
