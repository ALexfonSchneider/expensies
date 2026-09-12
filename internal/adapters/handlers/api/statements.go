package api

import (
	"errors"
	"fmt"
	"io"
	"net/http"
)

func (h *Handler) listStatements(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.ListStatements(r.Context())
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	out := make([]statementDTO, 0, len(items))
	for _, st := range items {
		out = append(out, h.statementDTO(st))
	}
	writeJSON(w, http.StatusOK, listDTO[statementDTO]{Items: out, Total: len(out)})
}

// uploadStatement accepts multipart/form-data with a "file" part.
func (h *Handler) uploadStatement(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, h.maxUpload)
	if err := r.ParseMultipartForm(h.maxUpload); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			h.writeError(w, r, err)
			return
		}
		h.badRequest(w, r, "expected multipart form: %v", err)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		h.badRequest(w, r, "file field is required")
		return
	}
	// The part is an in-memory or temp-file reader; closing it cannot
	// fail in a way the client needs to know about.
	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(file)
	if err != nil {
		h.writeError(w, r, fmt.Errorf("read upload: %w", err))
		return
	}
	res, err := h.svc.ImportStatement(r.Context(), header.Filename, data)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, h.importResultDTO(res))
}

func (h *Handler) deleteStatement(w http.ResponseWriter, r *http.Request) {
	id, err := urlID(r)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	if err := h.svc.DeleteStatement(r.Context(), id); err != nil {
		h.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
