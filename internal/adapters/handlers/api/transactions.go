package api

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/ALexfonSchneider/expenses/internal/domain"
)

func (h *Handler) listTransactions(w http.ResponseWriter, r *http.Request) {
	var f domain.TransactionFilter
	var err error
	if f.From, err = queryDate(r, "from"); err != nil {
		h.writeError(w, r, err)
		return
	}
	if f.To, err = queryDate(r, "to"); err != nil {
		h.writeError(w, r, err)
		return
	}
	if f.ID, err = queryInt64Ptr(r, "id"); err != nil {
		h.writeError(w, r, err)
		return
	}
	if f.CategoryID, err = queryInt64Ptr(r, "category_id"); err != nil {
		h.writeError(w, r, err)
		return
	}
	if f.Limit, err = queryInt(r, "limit", 0); err != nil {
		h.writeError(w, r, err)
		return
	}
	if f.Offset, err = queryInt(r, "offset", 0); err != nil {
		h.writeError(w, r, err)
		return
	}
	f.Uncategorized = queryBool(r, "uncategorized")
	f.IncludeTransfers = queryBool(r, "include_transfers")
	f.Direction = domain.Direction(r.URL.Query().Get("direction"))
	f.Kind = domain.Kind(r.URL.Query().Get("kind"))
	f.Query = r.URL.Query().Get("q")

	items, total, err := h.svc.ListTransactions(r.Context(), f)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	out := make([]transactionDTO, 0, len(items))
	for _, t := range items {
		out = append(out, h.transactionDTO(t))
	}
	writeJSON(w, http.StatusOK, listDTO[transactionDTO]{Items: out, Total: total})
}

// patchTransaction accepts {"category_id": 5 | null, "note": "..."}. A key
// that is absent leaves the field untouched; category_id null clears it.
func (h *Handler) patchTransaction(w http.ResponseWriter, r *http.Request) {
	id, err := urlID(r)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	var raw map[string]json.RawMessage
	if err := decodeJSON(r, &raw); err != nil {
		h.writeError(w, r, err)
		return
	}

	var patch domain.TransactionPatch
	if v, ok := raw["category_id"]; ok {
		patch.SetCategory = true
		if !bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
			var cid int64
			if err := json.Unmarshal(v, &cid); err != nil {
				h.badRequest(w, r, "category_id must be an integer or null")
				return
			}
			patch.CategoryID = &cid
		}
	}
	if v, ok := raw["note"]; ok {
		var note string
		if err := json.Unmarshal(v, &note); err != nil {
			h.badRequest(w, r, "note must be a string")
			return
		}
		patch.Note = &note
	}
	if v, ok := raw["excluded"]; ok {
		var excluded bool
		if err := json.Unmarshal(v, &excluded); err != nil {
			h.badRequest(w, r, "excluded must be a boolean")
			return
		}
		patch.Excluded = &excluded
	}

	tx, err := h.svc.UpdateTransaction(r.Context(), id, patch)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, h.transactionDTO(*tx))
}
