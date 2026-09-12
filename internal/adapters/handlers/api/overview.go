package api

import (
	"net/http"

	"github.com/ALexfonSchneider/expenses/internal/domain"
)

type signalDTO struct {
	Kind     string `json:"kind"`
	Severity string `json:"severity"`
	Title    string `json:"title"`
	Detail   string `json:"detail"`
	Link     string `json:"link"`
}

type monthOverviewDTO struct {
	Month         string `json:"month"`
	DaysElapsed   int    `json:"days_elapsed"`
	DaysInMonth   int    `json:"days_in_month"`
	Spent         int64  `json:"spent"`
	Forecast      int64  `json:"forecast"`
	Limit         int64  `json:"limit"`
	PaceLimit     int64  `json:"pace_limit"`
	PrevSamePoint int64  `json:"prev_same_point"`
	PrevTotal     int64  `json:"prev_total"`
}

type weekOverviewDTO struct {
	From        string      `json:"from"`
	To          string      `json:"to"`
	Spent       int64       `json:"spent"`
	Days        []bucketDTO `json:"days"`
	TypicalWeek int64       `json:"typical_week"`
	WeeksInAvg  int         `json:"weeks_in_avg"`
}

type trendOverviewDTO struct {
	Months  []bucketDTO `json:"months"`
	Average int64       `json:"average"`
}

type overviewDTO struct {
	Today   string           `json:"today"`
	Month   monthOverviewDTO `json:"month"`
	Week    weekOverviewDTO  `json:"week"`
	Trend   trendOverviewDTO `json:"trend"`
	Signals []signalDTO      `json:"signals"`
	Recent  []transactionDTO `json:"recent"`
}

func buckets(in []domain.Bucket) []bucketDTO {
	out := make([]bucketDTO, 0, len(in))
	for _, b := range in {
		out = append(out, bucketDTOOf(b))
	}
	return out
}

func (h *Handler) overview(w http.ResponseWriter, r *http.Request) {
	ov, err := h.svc.Overview(r.Context())
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	signals := make([]signalDTO, 0, len(ov.Signals))
	for _, s := range ov.Signals {
		signals = append(signals, signalDTO{Kind: s.Kind, Severity: string(s.Severity), Title: s.Title, Detail: s.Detail, Link: s.Link})
	}
	recent := make([]transactionDTO, 0, len(ov.Recent))
	for _, t := range ov.Recent {
		recent = append(recent, h.transactionDTO(t))
	}
	writeJSON(w, http.StatusOK, overviewDTO{
		Today: fmtDate(ov.Today),
		Month: monthOverviewDTO{
			Month:         ov.Month.Month.Format(monthLayout),
			DaysElapsed:   ov.Month.DaysElapsed,
			DaysInMonth:   ov.Month.DaysInMonth,
			Spent:         int64(ov.Month.Spent),
			Forecast:      int64(ov.Month.Forecast),
			Limit:         int64(ov.Month.Limit),
			PaceLimit:     int64(ov.Month.PaceLimit),
			PrevSamePoint: int64(ov.Month.PrevSamePoint),
			PrevTotal:     int64(ov.Month.PrevTotal),
		},
		Week: weekOverviewDTO{
			From:        fmtDate(ov.Week.From),
			To:          fmtDate(ov.Week.To),
			Spent:       int64(ov.Week.Spent),
			Days:        buckets(ov.Week.Days),
			TypicalWeek: int64(ov.Week.TypicalWeek),
			WeeksInAvg:  ov.Week.WeeksInAvg,
		},
		Trend: trendOverviewDTO{
			Months:  buckets(ov.Trend.Months),
			Average: int64(ov.Trend.Average),
		},
		Signals: signals,
		Recent:  recent,
	})
}
