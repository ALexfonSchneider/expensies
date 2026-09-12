package yandexpdf

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ALexfonSchneider/expenses/internal/domain"
)

func it(x float64, s string) item {
	return item{x: x, text: s}
}

func rw(y int64, items ...item) row {
	return row{y: y, items: items}
}

// header mirrors the three-line table header printed on every page.
func header(top int64) []row {
	return []row{
		rw(top, it(20, "Описание операции"), it(210, "Дата и время"), it(312, "Дата"), it(358, "Карта"), it(415, "Сумма в валюте"), it(510, "Сумма в валюте")),
		rw(top-9, it(224, "операции"), it(290, "обработки"), it(438, "операции"), it(554, "ЭСП")),
		rw(top-19, it(242, "МСК"), it(312, "МСК")),
	}
}

// samplePages replicates the geometry of a real statement with invented
// personal data. Sums: expenses 667+1250+840+667 = 3424, income 500+15 = 515.
func samplePages() [][]row {
	page1 := []row{
		rw(812, it(453, "Исх. № 123456")),
		rw(799, it(20, "Выписка по договору")),
		rw(752, it(20, "АО «Яндекс Банк», лицензия Банка России № 3027 (далее — «Банк»),")),
		rw(673, it(20, "Выписка по Договору за период с 01.01.2026 по 31.01.2026")),
		rw(648, it(20, "Входящий остаток на 01.01.2026"), it(538, "37,96 ₽")),
	}
	page1 = append(page1, header(627)...)
	page1 = append(page1,
		rw(587, it(20, "Входящий перевод СБП, Иван"), it(211, "04.01.2026"), it(281, "04.01.2026"), it(432, "+500,00 ₽"), it(527, "+500,00 ₽")),
		rw(578, it(235, "в 10:17")),
		rw(576, it(20, "Иванович И., +7 900 000-00-00, Т-")),
		rw(564, it(20, "Банк")),
		rw(539, it(20, "Оплата товаров и услуг"), it(211, "10.01.2026"), it(281, "10.01.2026"), it(354, "*5811"), it(432, "–667,00 ₽"), it(527, "–667,00 ₽")),
		rw(530, it(235, "в 12:00")),
		rw(528, it(20, "YANDEX*5814*EDA")),
		rw(500, it(20, "Оплата Сбер QR (Изикофе"), it(211, "11.01.2026"), it(281, "11.01.2026"), it(424, "–1 250,00 ₽"), it(519, "–1 250,00 ₽")),
		rw(491, it(235, "в 09:03")),
		rw(489, it(20, "Art_P_QR)")),
		rw(460, it(20, "Оплата товаров и услуг E-"), it(211, "12.01.2026"), it(281, "13.01.2026"), it(354, "*3903"), it(432, "–840,00 ₽"), it(527, "–840,00 ₽")),
		rw(451, it(235, "в 23:27")),
		rw(449, it(20, "TRAFFIC.RU")),
		rw(420, it(20, "Оплата товаров и услуг"), it(211, "10.01.2026"), it(281, "10.01.2026"), it(354, "*5811"), it(432, "–667,00 ₽"), it(527, "–667,00 ₽")),
		rw(411, it(235, "в 12:00")),
		rw(409, it(20, "YANDEX*5814*EDA")),
		rw(32, it(503, "Страница 1 из 2")),
		rw(20, it(20, "Продолжение на следующей странице")),
	)

	page2 := header(811)
	page2 = append(page2,
		rw(771, it(20, "Компенсация покупок по карте Пэй"), it(211, "20.01.2026"), it(281, "20.01.2026"), it(432, "+15,00 ₽"), it(527, "+15,00 ₽")),
		rw(762, it(235, "в 00:05")),
		rw(760, it(20, "баллами Плюса")),
		rw(649, it(20, "Исходящий остаток за 31.01.2026"), it(524, "1 000,00 ₽")),
		rw(621, it(20, "Всего расходных операций"), it(508, "–3 424,00 ₽")),
		rw(593, it(20, "Всего приходных операций"), it(508, "+515,00 ₽")),
		rw(113, it(20, "С уважением,")),
		rw(79, it(465, "Яндекс Банк")),
		rw(32, it(503, "Страница 2 из 2")),
	)
	return [][]row{page1, page2}
}

func TestParsePages_SampleStatement(t *testing.T) {
	loc := time.FixedZone("MSK", 3*60*60)
	ps, err := parsePages(samplePages(), loc)
	if err != nil {
		t.Fatalf("parsePages: %v", err)
	}
	if len(ps.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", ps.Warnings)
	}

	st := ps.Statement
	if st.Bank != Bank {
		t.Errorf("bank = %q, want %q", st.Bank, Bank)
	}
	if st.Number != "123456" {
		t.Errorf("number = %q, want 123456", st.Number)
	}
	if !st.PeriodFrom.Equal(domain.Date(2026, 1, 1)) || !st.PeriodTo.Equal(domain.Date(2026, 1, 31)) {
		t.Errorf("period = %v .. %v", st.PeriodFrom, st.PeriodTo)
	}
	if st.OpeningBalance != 3796 {
		t.Errorf("opening balance = %d, want 3796", st.OpeningBalance)
	}
	if st.ClosingBalance != 100000 {
		t.Errorf("closing balance = %d, want 100000", st.ClosingBalance)
	}
	if st.TotalExpense != 342400 || st.TotalIncome != 51500 {
		t.Errorf("totals = %d / %d, want 342400 / 51500", st.TotalExpense, st.TotalIncome)
	}
	if st.TxCount != 6 || len(ps.Transactions) != 6 {
		t.Fatalf("tx count = %d (%d), want 6", st.TxCount, len(ps.Transactions))
	}

	txs := ps.Transactions

	in := txs[0]
	if in.Kind != domain.KindTransferIn || in.Direction != domain.DirectionIncome || in.Amount != 50000 {
		t.Errorf("tx0 = %+v", in)
	}
	if in.Counterparty != "Иван Иванович И., +7 900 000-00-00, Т-Банк" {
		t.Errorf("tx0 counterparty = %q", in.Counterparty)
	}
	if want := time.Date(2026, 1, 4, 10, 17, 0, 0, loc); !in.OpAt.Equal(want) {
		t.Errorf("tx0 op_at = %v, want %v", in.OpAt, want)
	}
	if in.Card != "" {
		t.Errorf("tx0 card = %q, want empty", in.Card)
	}

	eda := txs[1]
	if eda.Kind != domain.KindPurchase || eda.Direction != domain.DirectionExpense || eda.Amount != 66700 {
		t.Errorf("tx1 = %+v", eda)
	}
	if eda.Merchant != "YANDEX*5814*EDA" || eda.MCC != "5814" || eda.Card != "*5811" {
		t.Errorf("tx1 merchant/mcc/card = %q/%q/%q", eda.Merchant, eda.MCC, eda.Card)
	}
	if eda.Description != "Оплата товаров и услуг YANDEX*5814*EDA" {
		t.Errorf("tx1 description = %q", eda.Description)
	}

	coffee := txs[2]
	if coffee.Merchant != "Изикофе Art" || coffee.Amount != 125000 {
		t.Errorf("tx2 merchant/amount = %q/%d", coffee.Merchant, coffee.Amount)
	}

	isp := txs[3]
	if isp.Merchant != "E-TRAFFIC.RU" || isp.Card != "*3903" {
		t.Errorf("tx3 merchant/card = %q/%q", isp.Merchant, isp.Card)
	}
	if !isp.ProcessedOn.Equal(domain.Date(2026, 1, 13)) {
		t.Errorf("tx3 processed_on = %v", isp.ProcessedOn)
	}

	if txs[4].Fingerprint == eda.Fingerprint {
		t.Errorf("identical operations must get distinct fingerprints")
	}

	cb := txs[5]
	if cb.Kind != domain.KindCashback || cb.Direction != domain.DirectionIncome || cb.Amount != 1500 {
		t.Errorf("tx5 = %+v", cb)
	}
	if cb.Description != "Компенсация покупок по карте Пэй баллами Плюса" {
		t.Errorf("tx5 description = %q", cb.Description)
	}

	seen := map[string]bool{}
	for _, tx := range txs {
		if seen[tx.Fingerprint] {
			t.Errorf("duplicate fingerprint %s", tx.Fingerprint)
		}
		seen[tx.Fingerprint] = true
	}
}

func TestParsePages_TotalsMismatchWarns(t *testing.T) {
	pages := samplePages()
	last := pages[1]
	for i := range last {
		if last[i].first() == "Всего расходных операций" {
			last[i].items[1].text = "–9 999,00 ₽"
		}
	}
	ps, err := parsePages(pages, time.UTC)
	if err != nil {
		t.Fatalf("parsePages: %v", err)
	}
	if len(ps.Warnings) != 1 || !strings.Contains(ps.Warnings[0], "expenses") {
		t.Fatalf("warnings = %v, want one expenses mismatch", ps.Warnings)
	}
}

func TestParsePages_RejectsForeignLayout(t *testing.T) {
	pages := [][]row{{
		rw(800, it(20, "Some Other Bank statement")),
		rw(700, it(20, "Описание операции"), it(300, "Дата и время"), it(400, "Сумма в валюте")),
	}}
	_, err := parsePages(pages, time.UTC)
	if err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("err = %v, want unsupported layout", err)
	}
}

func TestParseAmount(t *testing.T) {
	cases := []struct {
		in     string
		amount domain.Money
		sign   int
		ok     bool
	}{
		{"+500,00 ₽", 50000, 1, true},
		{"–138,00 ₽", 13800, -1, true},
		{"-138,00 ₽", 13800, -1, true},
		{"−138,00 ₽", 13800, -1, true},
		{"+1 254,00 ₽", 125400, 1, true},
		{"–371 220,61 ₽", 37122061, -1, true},
		{"37,96 ₽", 3796, 0, true},
		{"5 655,58", 565558, 0, true},
		{"abc", 0, 0, false},
		{"04.01.2026", 0, 0, false},
	}
	for _, c := range cases {
		amount, sign, err := parseAmount(c.in)
		if (err == nil) != c.ok {
			t.Errorf("%q: err = %v, ok = %v", c.in, err, c.ok)
			continue
		}
		if amount != c.amount || sign != c.sign {
			t.Errorf("%q: got %d/%d, want %d/%d", c.in, amount, sign, c.amount, c.sign)
		}
	}
}

func TestJoinLines(t *testing.T) {
	cases := []struct {
		in   []string
		want string
	}{
		{[]string{"Входящий перевод СБП, Иван", "Иванович И., +7 900 000-00-00, Т-", "Банк"}, "Входящий перевод СБП, Иван Иванович И., +7 900 000-00-00, Т-Банк"},
		{[]string{"Оплата товаров и услуг E-", "TRAFFIC.RU"}, "Оплата товаров и услуг E-TRAFFIC.RU"},
		{[]string{"Исходящий перевод СБП,", "Иван И., +7 900", "000-00-00, Сбербанк"}, "Исходящий перевод СБП, Иван И., +7 900 000-00-00, Сбербанк"},
		{[]string{"Оплата СБП QR (Гастроном  28_SBP)"}, "Оплата СБП QR (Гастроном 28_SBP)"},
		{[]string{"", "  "}, ""},
	}
	for _, c := range cases {
		if got := joinLines(c.in); got != c.want {
			t.Errorf("joinLines(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		desc         string
		kind         domain.Kind
		merchant     string
		mcc          string
		counterparty string
	}{
		{"Оплата товаров и услуг YANDEX*4121*TAXI", domain.KindPurchase, "YANDEX*4121*TAXI", "4121", ""},
		{"Оплата товаров и услуг SBER*5411*SAMOKAT", domain.KindPurchase, "SBER*5411*SAMOKAT", "5411", ""},
		{"Оплата товаров и услуг Metrofitnes", domain.KindPurchase, "Metrofitnes", "", ""},
		{"Оплата СБП QR (Додо Пицца, Барнаул-1)", domain.KindPurchase, "Додо Пицца, Барнаул-1", "", ""},
		{"Оплата СБП QR (Гастроном 28_SBP)", domain.KindPurchase, "Гастроном 28", "", ""},
		{"Оплата Сбер QR (MAGNIT MM OBZOR_P_QR)", domain.KindPurchase, "MAGNIT MM OBZOR", "", ""},
		{"Оплата Сбер Bluetooth RENOIR", domain.KindPurchase, "RENOIR", "", ""},
		{"Входящий перевод СБП, Иван И., +7 900 000-00-00, Т-Банк", domain.KindTransferIn, "", "", "Иван И., +7 900 000-00-00, Т-Банк"},
		{"Исходящий перевод СБП, Иван И., +7 900 000-00-00, Сбербанк", domain.KindTransferOut, "", "", "Иван И., +7 900 000-00-00, Сбербанк"},
		{"Внутрибанковский перевод на +7 900 000-00-00, Иван И.", domain.KindTransferOut, "", "", "+7 900 000-00-00, Иван И."},
		{"Внутрибанковский перевод от +7 900 000-00-00, Иван И.", domain.KindTransferIn, "", "", "+7 900 000-00-00, Иван И."},
		{"Внутрибанковский перевод с 40817810000000000000, Иван И.", domain.KindTransferIn, "", "", "40817810000000000000, Иван И."},
		{"Компенсация покупок по карте Пэй баллами Плюса", domain.KindCashback, "", "", ""},
		{"Отмена оплаты услуг", domain.KindRefund, "", "", ""},
		{"Что-то новое", domain.KindOther, "", "", ""},
	}
	for _, c := range cases {
		kind, merchant, mcc, cp := classify(c.desc)
		if kind != c.kind || merchant != c.merchant || mcc != c.mcc || cp != c.counterparty {
			t.Errorf("classify(%q) = %s/%q/%q/%q, want %s/%q/%q/%q",
				c.desc, kind, merchant, mcc, cp, c.kind, c.merchant, c.mcc, c.counterparty)
		}
	}
}

// TestParse_RealStatement runs the full parser against a real PDF when
// EXPENSES_TEST_PDF points at one. Real statements carry personal data and
// are never committed, so the test is skipped by default.
func TestParse_RealStatement(t *testing.T) {
	path := os.Getenv("EXPENSES_TEST_PDF")
	if path == "" {
		t.Skip("EXPENSES_TEST_PDF is not set")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	p, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ps, err := p.Parse(context.Background(), data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(ps.Warnings) != 0 {
		t.Errorf("warnings: %v", ps.Warnings)
	}
	if ps.Statement.Number == "" || ps.Statement.PeriodFrom.IsZero() {
		t.Errorf("statement header incomplete: %+v", ps.Statement)
	}
	if len(ps.Transactions) == 0 {
		t.Fatalf("no transactions parsed")
	}
	t.Logf("statement %s: %d operations, expenses %s, income %s",
		ps.Statement.Number, len(ps.Transactions),
		formatMoney(ps.Statement.TotalExpense), formatMoney(ps.Statement.TotalIncome))
}
