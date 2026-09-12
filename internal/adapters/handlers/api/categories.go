package api

import (
	"net/http"

	"github.com/ALexfonSchneider/expenses/internal/domain"
)

type categoryRequest struct {
	Name       string `json:"name"`
	Color      string `json:"color"`
	IsTransfer bool   `json:"is_transfer"`
	SortOrder  int    `json:"sort_order"`
}

type ruleRequest struct {
	CategoryID int64  `json:"category_id"`
	Field      string `json:"field"`
	Op         string `json:"op"`
	Value      string `json:"value"`
	Priority   int    `json:"priority"`
}

func (h *Handler) listCategories(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListCategories(r.Context())
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	out := make([]categoryDTO, 0, len(items))
	for _, c := range items {
		out = append(out, categoryDTOOf(c))
	}
	writeJSON(w, http.StatusOK, listDTO[categoryDTO]{Items: out, Total: len(out)})
}

func (h *Handler) createCategory(w http.ResponseWriter, r *http.Request) {
	var req categoryRequest
	if err := decodeJSON(r, &req); err != nil {
		h.writeError(w, r, err)
		return
	}
	c := domain.Category{Name: req.Name, Color: req.Color, IsTransfer: req.IsTransfer, SortOrder: req.SortOrder}
	if err := h.svc.CreateCategory(r.Context(), &c); err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, categoryDTOOf(c))
}

func (h *Handler) updateCategory(w http.ResponseWriter, r *http.Request) {
	id, err := urlID(r)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	var req categoryRequest
	if err := decodeJSON(r, &req); err != nil {
		h.writeError(w, r, err)
		return
	}
	c := domain.Category{ID: id, Name: req.Name, Color: req.Color, IsTransfer: req.IsTransfer, SortOrder: req.SortOrder}
	if err := h.svc.UpdateCategory(r.Context(), &c); err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, categoryDTOOf(c))
}

func (h *Handler) deleteCategory(w http.ResponseWriter, r *http.Request) {
	id, err := urlID(r)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	if err := h.svc.DeleteCategory(r.Context(), id); err != nil {
		h.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listRules(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListRules(r.Context())
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	out := make([]ruleDTO, 0, len(items))
	for _, rule := range items {
		out = append(out, ruleDTOOf(rule))
	}
	writeJSON(w, http.StatusOK, listDTO[ruleDTO]{Items: out, Total: len(out)})
}

func (h *Handler) createRule(w http.ResponseWriter, r *http.Request) {
	var req ruleRequest
	if err := decodeJSON(r, &req); err != nil {
		h.writeError(w, r, err)
		return
	}
	rule := domain.Rule{
		CategoryID: req.CategoryID,
		Field:      domain.RuleField(req.Field),
		Op:         domain.RuleOp(req.Op),
		Value:      req.Value,
		Priority:   req.Priority,
	}
	if err := h.svc.CreateRule(r.Context(), &rule); err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, ruleDTOOf(rule))
}

func (h *Handler) deleteRule(w http.ResponseWriter, r *http.Request) {
	id, err := urlID(r)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	if err := h.svc.DeleteRule(r.Context(), id); err != nil {
		h.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) applyRules(w http.ResponseWriter, r *http.Request) {
	n, err := h.svc.ApplyRules(r.Context())
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"updated": n})
}
