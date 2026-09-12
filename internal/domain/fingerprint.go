package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// Fingerprint derives a stable identity for a statement line. seq
// disambiguates identical operations within one statement (two equal
// purchases in the same minute) while still matching the same line when
// an overlapping statement is uploaded later.
func Fingerprint(bank string, opAt time.Time, dir Direction, amount Money, description string, seq int) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s|%s|%s|%d|%s|%d", bank, opAt.UTC().Format(time.RFC3339), dir, amount, description, seq)
	return hex.EncodeToString(h.Sum(nil))[:32]
}

// Date builds a calendar date as midnight UTC. Calendar dates are
// represented this way everywhere so comparisons do not depend on the
// wall-clock location.
func Date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// DateOf returns the calendar date of t in its own location.
func DateOf(t time.Time) time.Time {
	y, m, d := t.Date()
	return Date(y, m, d)
}
