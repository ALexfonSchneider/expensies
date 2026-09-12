package app

import (
	"testing"

	"github.com/ALexfonSchneider/expenses/internal/domain"
)

func idPtr(v int64) *int64 {
	return &v
}

func TestMergeCategoryDeltas(t *testing.T) {
	cur := []domain.CategoryTotal{
		{CategoryID: idPtr(1), Name: "Еда", Expense: 5000},
		{CategoryID: idPtr(2), Name: "Такси", Expense: 1000},
		{CategoryID: nil, Name: "", Expense: 300},
	}
	prev := []domain.CategoryTotal{
		{CategoryID: idPtr(1), Name: "Еда", Expense: 2000},
		{CategoryID: idPtr(3), Name: "Спорт", Expense: 1500},
		{CategoryID: idPtr(4), Name: "Пусто", Expense: 0},
	}
	got := mergeCategoryDeltas(cur, prev)
	if len(got) != 4 {
		t.Fatalf("len = %d, want 4 (zero-zero rows dropped): %+v", len(got), got)
	}
	if got[0].Name != "Еда" || got[0].Current != 5000 || got[0].Previous != 2000 {
		t.Errorf("first = %+v, want the biggest movement", got[0])
	}
	if got[1].Name != "Спорт" || got[1].Current != 0 || got[1].Previous != 1500 {
		t.Errorf("second = %+v", got[1])
	}
	if got[3].CategoryID != nil || got[3].Current != 300 {
		t.Errorf("uncategorized = %+v", got[3])
	}
}

func TestMergeMerchantDeltas(t *testing.T) {
	cur := []domain.MerchantTotal{{Merchant: "A", Expense: 100}, {Merchant: "B", Expense: 900}}
	prev := []domain.MerchantTotal{{Merchant: "A", Expense: 600}, {Merchant: "C", Expense: 50}}
	got := mergeMerchantDeltas(cur, prev, 2)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (capped)", len(got))
	}
	if got[0].Merchant != "B" || got[1].Merchant != "A" {
		t.Errorf("order = %+v", got)
	}
}

func TestPreviousRange(t *testing.T) {
	q := domain.RangeQuery{From: domain.Date(2026, 9, 1), To: domain.Date(2026, 9, 30), IncludeTransfers: true}
	prev := previousRange(q)
	if !prev.From.Equal(domain.Date(2026, 8, 2)) || !prev.To.Equal(domain.Date(2026, 8, 31)) || !prev.IncludeTransfers {
		t.Errorf("previousRange = %+v", prev)
	}
}
