package lkdr

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ALexfonSchneider/expenses/internal/domain"
)

func TestParseMoney(t *testing.T) {
	cases := []struct {
		in   string
		want domain.Money
		ok   bool
	}{
		{"1234.56", 123456, true},
		{"1 234,5", 123450, true},
		{"500", 50000, true},
		{"0.7", 70, true},
		{"-12.34", -1234, true},
		{"12.345", 1234, true},
		{"", 0, false},
		{"abc", 0, false},
	}
	for _, c := range cases {
		got, err := parseMoney(c.in)
		if (err == nil) != c.ok || got != c.want {
			t.Errorf("parseMoney(%q) = %d, %v; want %d, ok=%v", c.in, got, err, c.want, c.ok)
		}
	}
}

func TestFiscalScale(t *testing.T) {
	if got := fiscalScale(1234.56, 123456); got != 100 {
		t.Errorf("rubles document: scale = %v, want 100", got)
	}
	if got := fiscalScale(123456, 123456); got != 1 {
		t.Errorf("kopecks document: scale = %v, want 1", got)
	}
	if got := fiscalScale(99, 0); got != 100 {
		t.Errorf("unknown listing total: scale = %v, want 100", got)
	}
}

type memStore struct {
	s *domain.ReceiptSession
}

func (m *memStore) Load(context.Context) (*domain.ReceiptSession, error) {
	return m.s, nil
}

func (m *memStore) Save(_ context.Context, s *domain.ReceiptSession) error {
	cp := *s
	m.s = &cp
	return nil
}

func (m *memStore) Clear(context.Context) error {
	m.s = nil
	return nil
}

func TestSource_NoSession(t *testing.T) {
	src, err := New(&memStore{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := src.Probe(context.Background()); !errors.Is(err, domain.ErrReceiptSession) {
		t.Fatalf("Probe without session = %v, want ErrReceiptSession", err)
	}
}

// hangingTransport never answers until the request context is cancelled,
// which is how the archive behaves for clients outside Russia.
type hangingTransport struct{}

func (hangingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	<-req.Context().Done()
	return nil, req.Context().Err()
}

func TestSource_CallTimeout(t *testing.T) {
	store := &memStore{s: &domain.ReceiptSession{Phone: "79990000000", DeviceID: "d", RefreshToken: "r1"}}
	src, err := New(store, WithTransport(hangingTransport{}), WithCallTimeout(150*time.Millisecond))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	start := time.Now()
	err = src.Probe(context.Background())
	if err == nil {
		t.Fatalf("Probe against a hanging archive must fail")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("Probe took %v, the call timeout did not apply", elapsed)
	}
	if !strings.Contains(err.Error(), "did not answer") {
		t.Errorf("err = %v, want a timeout explanation", err)
	}
}

func TestTokenStorage_RoundTrip(t *testing.T) {
	store := &memStore{s: &domain.ReceiptSession{Phone: "79990000000", RefreshToken: "r1"}}
	now := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	ts := tokenStorage{store: store, now: func() time.Time { return now }}

	tokens, err := ts.LoadTokens(context.Background(), "79990000000")
	if err != nil || tokens == nil || tokens.RefreshToken != "r1" {
		t.Fatalf("LoadTokens = %+v, %v", tokens, err)
	}
	if !tokens.TokenExpireIn.Time().IsZero() {
		t.Errorf("a pasted session must force a refresh, got expiry %v", tokens.TokenExpireIn.Time())
	}
	if tokens, _ := ts.LoadTokens(context.Background(), "70000000000"); tokens != nil {
		t.Errorf("tokens for another phone must be nil")
	}
	if err := ts.UpdateTokens(context.Background(), "79990000000", nil); err != nil {
		t.Fatalf("UpdateTokens(nil): %v", err)
	}
	if store.s != nil {
		t.Errorf("nil tokens must clear the session")
	}
}
