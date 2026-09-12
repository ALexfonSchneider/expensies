package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ALexfonSchneider/goplatform/pkg/platform"

	"github.com/ALexfonSchneider/expenses/internal/domain"
)

const (
	dateLayout  = "2006-01-02"
	maxJSONBody = 1 << 20
)

type errorDTO struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	// A failure here means the client went away mid-response; nothing is
	// left to report to it and the request log already has the status.
	_ = json.NewEncoder(w).Encode(v)
}

func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	status, code := statusOf(err)
	msg := err.Error()
	if status >= http.StatusInternalServerError {
		h.logger.ErrorContext(r.Context(), "request failed",
			"method", r.Method, "path", r.URL.Path, "error", err)
		msg = "internal error"
	}
	writeJSON(w, status, errorDTO{Error: msg, Code: code})
}

func (h *Handler) badRequest(w http.ResponseWriter, r *http.Request, format string, args ...any) {
	h.writeError(w, r, fmt.Errorf("%w: %s", domain.ErrInvalid, fmt.Sprintf(format, args...)))
}

// statusOf maps domain sentinels first and platform error codes second, so
// both styles of error used across the SDK land on the right status.
func statusOf(err error) (int, string) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		return http.StatusNotFound, "not_found"
	case errors.Is(err, domain.ErrAlreadyExists):
		return http.StatusConflict, "already_exists"
	case errors.Is(err, domain.ErrInvalid):
		return http.StatusBadRequest, "invalid_argument"
	case errors.Is(err, domain.ErrReceiptSession):
		return http.StatusPreconditionFailed, "receipt_session"
	}
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return http.StatusRequestEntityTooLarge, "too_large"
	}
	switch platform.GetCode(err) {
	case platform.CodeNotFound:
		return http.StatusNotFound, "not_found"
	case platform.CodeInvalidArgument:
		return http.StatusBadRequest, "invalid_argument"
	case platform.CodeAlreadyExists:
		return http.StatusConflict, "already_exists"
	case platform.CodePermissionDenied:
		return http.StatusForbidden, "permission_denied"
	case platform.CodeUnauthenticated:
		return http.StatusUnauthorized, "unauthenticated"
	case platform.CodeUnavailable:
		return http.StatusServiceUnavailable, "unavailable"
	}
	return http.StatusInternalServerError, "internal"
}

func decodeJSON(r *http.Request, v any) error {
	if err := json.NewDecoder(io.LimitReader(r.Body, maxJSONBody)).Decode(v); err != nil {
		return fmt.Errorf("%w: bad json: %v", domain.ErrInvalid, err)
	}
	return nil
}

func urlID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("%w: bad id", domain.ErrInvalid)
	}
	return id, nil
}

func queryDate(r *http.Request, name string) (*time.Time, error) {
	s := strings.TrimSpace(r.URL.Query().Get(name))
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return nil, fmt.Errorf("%w: %s must be YYYY-MM-DD", domain.ErrInvalid, name)
	}
	return &t, nil
}

func requireDate(r *http.Request, name string) (time.Time, error) {
	t, err := queryDate(r, name)
	if err != nil {
		return time.Time{}, err
	}
	if t == nil {
		return time.Time{}, fmt.Errorf("%w: %s is required", domain.ErrInvalid, name)
	}
	return *t, nil
}

func queryBool(r *http.Request, name string) bool {
	switch strings.ToLower(strings.TrimSpace(r.URL.Query().Get(name))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

func queryInt(r *http.Request, name string, def int) (int, error) {
	s := strings.TrimSpace(r.URL.Query().Get(name))
	if s == "" {
		return def, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("%w: %s must be an integer", domain.ErrInvalid, name)
	}
	return n, nil
}

func queryInt64Ptr(r *http.Request, name string) (*int64, error) {
	s := strings.TrimSpace(r.URL.Query().Get(name))
	if s == "" {
		return nil, nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("%w: %s must be an integer", domain.ErrInvalid, name)
	}
	return &n, nil
}
