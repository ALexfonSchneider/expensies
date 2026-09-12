package app

import (
	"testing"

	"github.com/ALexfonSchneider/expenses/internal/domain"
)

func TestFormatRubles(t *testing.T) {
	cases := map[domain.Money]string{
		0:          "0 ₽",
		99:         "0 ₽",
		123456:     "1 234 ₽",
		1234567890: "12 345 678 ₽",
		-500000:    "-5 000 ₽",
	}
	for in, want := range cases {
		if got := formatRubles(in); got != want {
			t.Errorf("formatRubles(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestSeverityRank(t *testing.T) {
	if severityRank(domain.SeverityBad) >= severityRank(domain.SeverityWarn) || severityRank(domain.SeverityWarn) >= severityRank(domain.SeverityInfo) {
		t.Errorf("severities must rank bad < warn < info")
	}
}

func TestContainsSessionError(t *testing.T) {
	if !containsSessionError("lkdr: list receipts: receipt service session is missing or expired") {
		t.Errorf("session error not detected")
	}
	if containsSessionError("archive did not answer within 1m30s") {
		t.Errorf("timeout must not look like a session error")
	}
}
