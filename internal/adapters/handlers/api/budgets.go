package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ALexfonSchneider/expenses/internal/domain"
)

const monthLayout = "2006-01"

type budgetRequest struct {
	CategoryID *int64 `json:"category_id"`
	Amount     int64  `json:"amount"`
}

func (h *Handler) listBudgets(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListBudgets(r.Context())
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	out := make([]budgetDTO, 0, len(items))
	for _, b := range items {
		out = append(out, budgetDTOOf(b))
	}
	writeJSON(w, http.StatusOK, listDTO[budgetDTO]{Items: out, Total: len(out)})
}

// setBudget creates or replaces the limit of a category; category_id null
// is the overall monthly limit.
func (h *Handler) setBudget(w http.ResponseWriter, r *http.Request) {
	var req budgetRequest
	if err := decodeJSON(r, &req); err != nil {
		h.writeError(w, r, err)
		return
	}
	b, err := h.svc.SetBudget(r.Context(), req.CategoryID, domain.Money(req.Amount))
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, budgetDTOOf(*b))
}

func (h *Handler) deleteBudget(w http.ResponseWriter, r *http.Request) {
	id, err := urlID(r)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	if err := h.svc.DeleteBudget(r.Context(), id); err != nil {
		h.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) budgetReport(w http.ResponseWriter, r *http.Request) {
	s := strings.TrimSpace(r.URL.Query().Get("month"))
	month, err := time.Parse(monthLayout, s)
	if err != nil {
		h.writeError(w, r, fmt.Errorf("%w: month must be YYYY-MM", domain.ErrInvalid))
		return
	}
	report, err := h.svc.BudgetReport(r.Context(), month)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, budgetReportDTOOf(report))
}
