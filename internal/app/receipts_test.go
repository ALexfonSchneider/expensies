package app

import (
	"errors"
	"testing"
	"time"

	"github.com/ALexfonSchneider/expenses/internal/domain"
)

func TestChooseCandidate(t *testing.T) {
	issued := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	tx := func(id int64, offset time.Duration) domain.Transaction {
		return domain.Transaction{ID: id, OpAt: issued.Add(offset)}
	}

	if _, ok := chooseCandidate(issued, nil); ok {
		t.Errorf("no candidates must not match")
	}
	if id, ok := chooseCandidate(issued, []domain.Transaction{tx(1, 2*time.Minute), tx(2, 40*time.Minute)}); !ok || id != 1 {
		t.Errorf("nearest within tight window: got %d/%v", id, ok)
	}
	if id, ok := chooseCandidate(issued, []domain.Transaction{tx(3, -20*time.Hour)}); !ok || id != 3 {
		t.Errorf("lone candidate within loose window: got %d/%v", id, ok)
	}
	if _, ok := chooseCandidate(issued, []domain.Transaction{tx(4, 5*time.Hour), tx(5, -6*time.Hour)}); ok {
		t.Errorf("two far candidates are ambiguous and must not match")
	}
	if _, ok := chooseCandidate(issued, []domain.Transaction{tx(6, 3*24*time.Hour)}); ok {
		t.Errorf("candidate beyond the loose window must not match")
	}
}

func TestReceiptGroups(t *testing.T) {
	at := time.Date(2026, 9, 10, 10, 50, 0, 0, time.UTC)
	rc := func(id int64, inn string, offset time.Duration, total domain.Money) domain.Receipt {
		return domain.Receipt{ID: id, SellerINN: inn, IssuedAt: at.Add(offset), Total: total, OperationType: domain.ReceiptOpSale}
	}
	groups := receiptGroups([]domain.Receipt{
		rc(1, "770", 0, 82020),
		rc(2, "770", time.Minute, 4900),
		rc(3, "770", 2*time.Minute, 4100),
		rc(4, "770", 3*time.Hour, 500),
		rc(5, "111", 0, 100),
		rc(6, "", 0, 100),
		{ID: 7, SellerINN: "770", IssuedAt: at.Add(30 * time.Second), Total: 300, OperationType: domain.ReceiptOpSaleRefund},
	})
	if len(groups) != 1 {
		t.Fatalf("groups = %d, want 1: %+v", len(groups), groups)
	}
	ids := make([]int64, 0, 3)
	for _, r := range groups[0] {
		ids = append(ids, r.ID)
	}
	if len(ids) != 3 || ids[0] != 1 || ids[1] != 2 || ids[2] != 3 {
		t.Errorf("group ids = %v, want [1 2 3]", ids)
	}
}

func TestTransactionFromReceipt(t *testing.T) {
	r := domain.Receipt{
		Source:        "lkdr",
		Key:           "k1",
		SellerName:    `ОБЩЕСТВО С ОГРАНИЧЕННОЙ ОТВЕТСТВЕННОСТЬЮ "ЯНДЕКС.ЕДА"`,
		SellerINN:     "770",
		RetailPlace:   "",
		IssuedAt:      time.Date(2026, 9, 12, 13, 54, 0, 0, time.UTC),
		OperationType: domain.ReceiptOpSale,
		Total:         33400,
	}
	tx := transactionFromReceipt(r)
	if tx.Source != domain.SourceReceipt || tx.Fingerprint != "receipt:lkdr:k1" || tx.Amount != 33400 {
		t.Errorf("tx = %+v", tx)
	}
	if tx.Merchant != `ООО "ЯНДЕКС.ЕДА"` || tx.Direction != domain.DirectionExpense || tx.Kind != domain.KindPurchase {
		t.Errorf("merchant/direction/kind = %q/%s/%s", tx.Merchant, tx.Direction, tx.Kind)
	}
	r.OperationType = domain.ReceiptOpSaleRefund
	r.RetailPlace = "Магазин у дома"
	tx = transactionFromReceipt(r)
	if tx.Direction != domain.DirectionIncome || tx.Kind != domain.KindRefund || tx.Merchant != "Магазин у дома" {
		t.Errorf("refund tx = %+v", tx)
	}
}

func TestReversedReceiptsAndStandsAlone(t *testing.T) {
	at := time.Date(2026, 9, 1, 5, 51, 0, 0, time.UTC)
	paid := func(id int64, op int, offset time.Duration, total domain.Money) domain.Receipt {
		return domain.Receipt{ID: id, SellerINN: "781", OperationType: op, IssuedAt: at.Add(offset), Total: total, EcashTotal: total, ItemsLoaded: true}
	}
	sale := paid(1, domain.ReceiptOpSale, 0, 120400)
	refund := paid(2, domain.ReceiptOpSaleRefund, 2*time.Minute, 120400)
	kept := paid(3, domain.ReceiptOpSale, time.Hour, 97400)
	offset := domain.Receipt{ID: 4, SellerINN: "781", OperationType: domain.ReceiptOpSale, IssuedAt: at.Add(2 * time.Hour), Total: 97400, ItemsLoaded: true}
	realRefund := paid(5, domain.ReceiptOpSaleRefund, 3*24*time.Hour, 120400)
	pending := domain.Receipt{ID: 6, SellerINN: "781", OperationType: domain.ReceiptOpSale, IssuedAt: at, Total: 500}

	reversed := reversedReceipts([]domain.Receipt{sale, refund, kept, offset, realRefund, pending})
	if !reversed[1] || !reversed[2] {
		t.Errorf("sale and its refund must be marked reversed: %v", reversed)
	}
	if reversed[3] || reversed[5] {
		t.Errorf("unrelated sale or a refund days later must not be reversed: %v", reversed)
	}

	if standsAlone(sale, reversed) || standsAlone(refund, reversed) {
		t.Errorf("a reversed pair must not stand alone")
	}
	if !standsAlone(kept, reversed) {
		t.Errorf("a paid sale must stand alone")
	}
	if standsAlone(offset, reversed) {
		t.Errorf("a zero-payment receipt (prepayment offset) must not stand alone")
	}
	if !standsAlone(realRefund, reversed) {
		t.Errorf("a refund with money returned must stand alone as income")
	}
	if standsAlone(pending, reversed) {
		t.Errorf("a receipt without details must wait")
	}
}

func TestShortLegalName(t *testing.T) {
	cases := map[string]string{
		`ОБЩЕСТВО С ОГРАНИЧЕННОЙ ОТВЕТСТВЕННОСТЬЮ "РОМАШКА"`: `ООО "РОМАШКА"`,
		"Индивидуальный предприниматель Иванов И.И.":         "ИП Иванов И.И.",
		"АКЦИОНЕРНОЕ ОБЩЕСТВО ТАНДЕР":                        "АО ТАНДЕР",
		"Самокат": "Самокат",
	}
	for in, want := range cases {
		if got := ShortLegalName(in); got != want {
			t.Errorf("ShortLegalName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizePhone(t *testing.T) {
	good := map[string]string{
		"+7 999 123-45-67": "79991234567",
		"89991234567":      "79991234567",
		"9991234567":       "79991234567",
		"79991234567":      "79991234567",
	}
	for in, want := range good {
		got, err := normalizePhone(in)
		if err != nil || got != want {
			t.Errorf("normalizePhone(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"", "12345", "+1 202 555 0100", "7999"} {
		if _, err := normalizePhone(in); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("normalizePhone(%q) err = %v, want ErrInvalid", in, err)
		}
	}
}
