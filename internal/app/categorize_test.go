package app

import (
	"errors"
	"testing"

	"github.com/ALexfonSchneider/expenses/internal/domain"
)

func TestCategorizer_PriorityAndOps(t *testing.T) {
	rules := []domain.Rule{
		{ID: 1, CategoryID: 10, Field: domain.RuleFieldMCC, Op: domain.RuleOpEquals, Value: "5814", Priority: 40},
		{ID: 2, CategoryID: 20, Field: domain.RuleFieldMerchant, Op: domain.RuleOpContains, Value: "eda", Priority: 10},
		{ID: 3, CategoryID: 30, Field: domain.RuleFieldKind, Op: domain.RuleOpEquals, Value: "transfer_in", Priority: 5},
		{ID: 4, CategoryID: 40, Field: domain.RuleFieldDescription, Op: domain.RuleOpRegex, Value: `^оплата сбп qr \(додо`, Priority: 50},
		{ID: 5, CategoryID: 50, Field: domain.RuleFieldMerchant, Op: domain.RuleOpPrefix, Value: "sber*", Priority: 60},
	}
	c, err := NewCategorizer(rules)
	if err != nil {
		t.Fatalf("NewCategorizer: %v", err)
	}

	cases := []struct {
		name string
		tx   domain.Transaction
		want int64
		ok   bool
	}{
		{"merchant contains beats mcc by priority", domain.Transaction{Merchant: "YANDEX*5814*EDA", MCC: "5814", Kind: domain.KindPurchase}, 20, true},
		{"mcc fallback", domain.Transaction{Merchant: "BURGER", MCC: "5814", Kind: domain.KindPurchase}, 10, true},
		{"kind equals", domain.Transaction{Kind: domain.KindTransferIn, Counterparty: "Иван"}, 30, true},
		{"regex is case-insensitive", domain.Transaction{Description: "Оплата СБП QR (Додо Пицца)", Kind: domain.KindPurchase}, 40, true},
		{"prefix", domain.Transaction{Merchant: "SBER*5411*SAMOKAT", Kind: domain.KindPurchase}, 50, true},
		{"no match", domain.Transaction{Merchant: "UNKNOWN", Kind: domain.KindPurchase}, 0, false},
	}
	for _, tc := range cases {
		got, ok := c.Match(&tc.tx)
		if got != tc.want || ok != tc.ok {
			t.Errorf("%s: got %d/%v, want %d/%v", tc.name, got, ok, tc.want, tc.ok)
		}
	}
}

func TestValidateRule(t *testing.T) {
	bad := []domain.Rule{
		{CategoryID: 1, Field: "nope", Op: domain.RuleOpContains, Value: "x"},
		{CategoryID: 1, Field: domain.RuleFieldMerchant, Op: "nope", Value: "x"},
		{CategoryID: 1, Field: domain.RuleFieldMerchant, Op: domain.RuleOpRegex, Value: "("},
		{CategoryID: 1, Field: domain.RuleFieldMerchant, Op: domain.RuleOpContains, Value: "  "},
		{CategoryID: 0, Field: domain.RuleFieldMerchant, Op: domain.RuleOpContains, Value: "x"},
	}
	for i, r := range bad {
		if err := ValidateRule(r); !errors.Is(err, domain.ErrInvalid) {
			t.Errorf("rule %d: err = %v, want ErrInvalid", i, err)
		}
	}
	good := domain.Rule{CategoryID: 1, Field: domain.RuleFieldMCC, Op: domain.RuleOpEquals, Value: "5411"}
	if err := ValidateRule(good); err != nil {
		t.Errorf("good rule rejected: %v", err)
	}
}
