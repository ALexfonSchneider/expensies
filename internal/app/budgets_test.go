package app

import (
	"testing"

	"github.com/ALexfonSchneider/expenses/internal/domain"
)

func TestForecast(t *testing.T) {
	cases := []struct {
		spent         domain.Money
		elapsed, days int
		want          domain.Money
	}{
		{0, 0, 30, 0},
		{1000, 0, 30, 0},
		{1000, 10, 30, 3000},
		{1000, 30, 30, 1000},
		{1000, 31, 30, 1000},
		{700, 7, 31, 3100},
	}
	for _, c := range cases {
		if got := forecast(c.spent, c.elapsed, c.days); got != c.want {
			t.Errorf("forecast(%d, %d, %d) = %d, want %d", c.spent, c.elapsed, c.days, got, c.want)
		}
	}
}

func TestPaceLimit(t *testing.T) {
	if got := paceLimit(3000, 10, 30); got != 1000 {
		t.Errorf("paceLimit = %d, want 1000", got)
	}
	if got := paceLimit(3000, 0, 30); got != 0 {
		t.Errorf("paceLimit at day 0 = %d, want 0", got)
	}
	if got := paceLimit(3000, 30, 30); got != 3000 {
		t.Errorf("paceLimit at month end = %d, want 3000", got)
	}
}
