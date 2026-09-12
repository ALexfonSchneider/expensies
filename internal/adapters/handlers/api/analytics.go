package api

import (
	"net/http"

	"github.com/ALexfonSchneider/expenses/internal/domain"
)

func rangeQuery(r *http.Request) (domain.RangeQuery, error) {
	from, err := requireDate(r, "from")
	if err != nil {
		return domain.RangeQuery{}, err
	}
	to, err := requireDate(r, "to")
	if err != nil {
		return domain.RangeQuery{}, err
	}
	return domain.RangeQuery{From: from, To: to, IncludeTransfers: queryBool(r, "include_transfers")}, nil
}

func (h *Handler) summary(w http.ResponseWriter, r *http.Request) {
	q, err := rangeQuery(r)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	s, err := h.svc.Summary(r.Context(), q)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, summaryDTOOf(s))
}

func (h *Handler) series(w http.ResponseWriter, r *http.Request) {
	q, err := rangeQuery(r)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	g := domain.Granularity(r.URL.Query().Get("granularity"))
	if g == "" {
		g = domain.GranularityDay
	}
	items, err := h.svc.Series(r.Context(), domain.SeriesQuery{RangeQuery: q, Granularity: g})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	out := make([]bucketDTO, 0, len(items))
	for _, b := range items {
		out = append(out, bucketDTOOf(b))
	}
	writeJSON(w, http.StatusOK, seriesDTO{Granularity: string(g), Items: out})
}

func (h *Handler) byCategory(w http.ResponseWriter, r *http.Request) {
	q, err := rangeQuery(r)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	items, err := h.svc.ByCategory(r.Context(), q)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	out := make([]categoryTotalDTO, 0, len(items))
	for _, ct := range items {
		out = append(out, categoryTotalDTO{
			CategoryID: ct.CategoryID,
			Name:       ct.Name,
			Color:      ct.Color,
			IsTransfer: ct.IsTransfer,
			Expense:    int64(ct.Expense),
			Income:     int64(ct.Income),
			Count:      ct.Count,
		})
	}
	writeJSON(w, http.StatusOK, listDTO[categoryTotalDTO]{Items: out, Total: len(out)})
}

func (h *Handler) topMerchants(w http.ResponseWriter, r *http.Request) {
	q, err := rangeQuery(r)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	limit, err := queryInt(r, "limit", 0)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	items, err := h.svc.TopMerchants(r.Context(), q, limit)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	out := make([]merchantTotalDTO, 0, len(items))
	for _, mt := range items {
		out = append(out, merchantTotalDTO{Merchant: mt.Merchant, Expense: int64(mt.Expense), Count: mt.Count})
	}
	writeJSON(w, http.StatusOK, listDTO[merchantTotalDTO]{Items: out, Total: len(out)})
}

func (h *Handler) compare(w http.ResponseWriter, r *http.Request) {
	q, err := rangeQuery(r)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	c, err := h.svc.Compare(r.Context(), q)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, comparisonDTOOf(c))
}

func (h *Handler) dataRange(w http.ResponseWriter, r *http.Request) {
	dr, err := h.svc.DataRange(r.Context())
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, rangeDTO{From: fmtDate(dr.From), To: fmtDate(dr.To), HasData: dr.HasData})
}
