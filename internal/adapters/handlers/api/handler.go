// Package api exposes the use cases as a JSON HTTP API. The composition
// root decides the mount path (normally /api/v1).
package api

import (
	"fmt"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/ALexfonSchneider/goplatform/pkg/platform"

	"github.com/ALexfonSchneider/expenses/internal/app"
)

const defaultMaxUpload = 20 << 20

// Handler is the primary HTTP adapter.
type Handler struct {
	svc       *app.Service
	logger    platform.Logger
	loc       *time.Location
	maxUpload int64
}

// Option configures a Handler.
type Option func(*Handler)

// WithMaxUpload caps the size of an uploaded statement in bytes.
func WithMaxUpload(bytes int64) Option {
	return func(h *Handler) {
		if bytes > 0 {
			h.maxUpload = bytes
		}
	}
}

// WithLocation sets the zone used to render operation timestamps.
func WithLocation(loc *time.Location) Option {
	return func(h *Handler) {
		if loc != nil {
			h.loc = loc
		}
	}
}

// New creates a Handler.
func New(svc *app.Service, logger platform.Logger, opts ...Option) (*Handler, error) {
	if svc == nil {
		return nil, fmt.Errorf("api: service is required")
	}
	if logger == nil {
		logger = platform.NopLogger()
	}
	h := &Handler{
		svc:       svc,
		logger:    logger,
		loc:       time.FixedZone("MSK", 3*60*60),
		maxUpload: defaultMaxUpload,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h, nil
}

// Routes returns the router with every endpoint, relative to the mount.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/overview", h.overview)
	r.Route("/statements", func(r chi.Router) {
		r.Get("/", h.listStatements)
		r.Post("/", h.uploadStatement)
		r.Delete("/{id}", h.deleteStatement)
	})
	r.Route("/transactions", func(r chi.Router) {
		r.Get("/", h.listTransactions)
		r.Patch("/{id}", h.patchTransaction)
		r.Get("/{id}/receipts", h.transactionReceipts)
	})
	r.Route("/receipts", func(r chi.Router) {
		r.Get("/session", h.getReceiptSession)
		r.Put("/session", h.putReceiptSession)
		r.Delete("/session", h.deleteReceiptSession)
		r.Get("/sync", h.receiptSyncStatus)
		r.Post("/sync", h.startReceiptSync)
		r.Post("/match", h.matchReceipts)
		r.Get("/", h.listReceipts)
		r.Get("/{id}", h.getReceipt)
		r.Get("/{id}/candidates", h.receiptCandidates)
		r.Post("/{id}/link", h.linkReceipt)
		r.Delete("/{id}/link", h.unlinkReceipt)
	})
	r.Route("/categories", func(r chi.Router) {
		r.Get("/", h.listCategories)
		r.Post("/", h.createCategory)
		r.Put("/{id}", h.updateCategory)
		r.Delete("/{id}", h.deleteCategory)
	})
	r.Route("/rules", func(r chi.Router) {
		r.Get("/", h.listRules)
		r.Post("/", h.createRule)
		r.Post("/apply", h.applyRules)
		r.Delete("/{id}", h.deleteRule)
	})
	r.Route("/budgets", func(r chi.Router) {
		r.Get("/", h.listBudgets)
		r.Put("/", h.setBudget)
		r.Get("/report", h.budgetReport)
		r.Delete("/{id}", h.deleteBudget)
	})
	r.Route("/analytics", func(r chi.Router) {
		r.Get("/summary", h.summary)
		r.Get("/series", h.series)
		r.Get("/categories", h.byCategory)
		r.Get("/merchants", h.topMerchants)
		r.Get("/compare", h.compare)
		r.Get("/items", h.topItems)
		r.Get("/range", h.dataRange)
	})
	return r
}
