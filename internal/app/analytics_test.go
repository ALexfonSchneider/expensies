package app

import (
	"testing"
	"time"

	"github.com/ALexfonSchneider/expenses/internal/domain"
)

func TestTruncateTo(t *testing.T) {
	// 2026-09-12 is a Saturday.
	sat := domain.Date(2026, 9, 12)
	cases := []struct {
		g    domain.Granularity
		want time.Time
	}{
		{domain.GranularityDay, domain.Date(2026, 9, 12)},
		{domain.GranularityWeek, domain.Date(2026, 9, 7)},
		{domain.GranularityMonth, domain.Date(2026, 9, 1)},
	}
	for _, c := range cases {
		if got := truncateTo(sat, c.g); !got.Equal(c.want) {
			t.Errorf("truncateTo(%s) = %v, want %v", c.g, got, c.want)
		}
	}
	// A Monday must stay on itself and a Sunday must go back six days.
	if got := truncateTo(domain.Date(2026, 9, 7), domain.GranularityWeek); !got.Equal(domain.Date(2026, 9, 7)) {
		t.Errorf("monday week start = %v", got)
	}
	if got := truncateTo(domain.Date(2026, 9, 13), domain.GranularityWeek); !got.Equal(domain.Date(2026, 9, 7)) {
		t.Errorf("sunday week start = %v", got)
	}
}

func TestFillGaps(t *testing.T) {
	raw := []domain.Bucket{
		{Start: domain.Date(2026, 1, 2), Expense: 100, Count: 1},
		{Start: domain.Date(2026, 1, 4), Income: 50, Count: 1},
	}
	got := fillGaps(raw, domain.Date(2026, 1, 1), domain.Date(2026, 1, 5), domain.GranularityDay)
	if len(got) != 5 {
		t.Fatalf("len = %d, want 5", len(got))
	}
	if got[0].Expense != 0 || got[1].Expense != 100 || got[3].Income != 50 || got[4].Count != 0 {
		t.Errorf("buckets = %+v", got)
	}

	weeks := fillGaps(nil, domain.Date(2026, 1, 1), domain.Date(2026, 1, 31), domain.GranularityWeek)
	if len(weeks) != 5 || !weeks[0].Start.Equal(domain.Date(2025, 12, 29)) {
		t.Errorf("weeks = %+v", weeks)
	}

	months := fillGaps(nil, domain.Date(2026, 1, 15), domain.Date(2026, 3, 2), domain.GranularityMonth)
	if len(months) != 3 || !months[2].Start.Equal(domain.Date(2026, 3, 1)) {
		t.Errorf("months = %+v", months)
	}
}

func TestDaysBetween(t *testing.T) {
	if got := daysBetween(domain.Date(2026, 1, 1), domain.Date(2026, 1, 1)); got != 1 {
		t.Errorf("same day = %d, want 1", got)
	}
	if got := daysBetween(domain.Date(2026, 1, 1), domain.Date(2026, 1, 31)); got != 31 {
		t.Errorf("january = %d, want 31", got)
	}
}
