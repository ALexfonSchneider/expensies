package domain

import "time"

// Signal is one thing on the overview that deserves attention, with a
// link to where it can be acted on.
type Signal struct {
	Kind     string
	Severity SignalSeverity
	Title    string
	Detail   string
	Link     string
}

// SignalSeverity orders signals: problems first, hints last.
type SignalSeverity string

const (
	SeverityBad  SignalSeverity = "bad"
	SeverityWarn SignalSeverity = "warn"
	SeverityInfo SignalSeverity = "info"
)

// MonthOverview is the state of the current calendar month.
type MonthOverview struct {
	Month       time.Time
	DaysElapsed int
	DaysInMonth int
	Spent       Money
	Forecast    Money
	Limit       Money
	PaceLimit   Money
	// PrevSamePoint is the previous month's spend up to the same day of
	// month; PrevTotal is its full amount.
	PrevSamePoint Money
	PrevTotal     Money
}

// WeekOverview is the last seven days against a typical week.
type WeekOverview struct {
	From        time.Time
	To          time.Time
	Spent       Money
	Days        []Bucket
	TypicalWeek Money
	WeeksInAvg  int
}

// TrendOverview is the last months with the average of the completed ones.
type TrendOverview struct {
	Months  []Bucket
	Average Money
}

// Overview is the home page: how things stand today.
type Overview struct {
	Today   time.Time
	Month   MonthOverview
	Week    WeekOverview
	Trend   TrendOverview
	Signals []Signal
	Recent  []Transaction
}
