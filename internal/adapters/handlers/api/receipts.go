package api

import (
	"net/http"

	"github.com/ALexfonSchneider/expenses/internal/domain"
)

type sessionRequest struct {
	Phone        string `json:"phone"`
	DeviceID     string `json:"device_id"`
	RefreshToken string `json:"refresh_token"`
	AccessToken  string `json:"access_token"`
}

type sessionDTO struct {
	Configured       bool   `json:"configured"`
	Phone            string `json:"phone"`
	DeviceID         string `json:"device_id"`
	TokenExpiresAt   string `json:"token_expires_at"`
	RefreshExpiresAt string `json:"refresh_expires_at"`
	UpdatedAt        string `json:"updated_at"`
}

type syncStatusDTO struct {
	Running    bool   `json:"running"`
	StartedAt  string `json:"started_at"`
	FinishedAt string `json:"finished_at"`
	Listed     int    `json:"listed"`
	Added      int    `json:"added"`
	Detailed   int    `json:"detailed"`
	Pending    int    `json:"pending"`
	Matched    int    `json:"matched"`
	Error      string `json:"error"`
}

type receiptItemDTO struct {
	Position   int     `json:"position"`
	Name       string  `json:"name"`
	Price      int64   `json:"price"`
	Quantity   float64 `json:"quantity"`
	Sum        int64   `json:"sum"`
	CategoryID *int64  `json:"category_id"`
}

type receiptDTO struct {
	ID            int64            `json:"id"`
	Key           string           `json:"key"`
	SellerName    string           `json:"seller_name"`
	SellerINN     string           `json:"seller_inn"`
	RetailPlace   string           `json:"retail_place"`
	RetailAddress string           `json:"retail_address"`
	IssuedAt      string           `json:"issued_at"`
	IssuedOn      string           `json:"issued_on"`
	ReceivedAt    string           `json:"received_at"`
	OperationType int              `json:"operation_type"`
	Total         int64            `json:"total"`
	CashTotal     int64            `json:"cash_total"`
	EcashTotal    int64            `json:"ecash_total"`
	ItemsLoaded   bool             `json:"items_loaded"`
	ItemsError    string           `json:"items_error"`
	TransactionID *int64           `json:"transaction_id"`
	MatchKind     string           `json:"match_kind"`
	ItemCount     int              `json:"item_count"`
	Items         []receiptItemDTO `json:"items,omitempty"`
}

type itemTotalDTO struct {
	Name     string  `json:"name"`
	Quantity float64 `json:"quantity"`
	Sum      int64   `json:"sum"`
	Count    int     `json:"count"`
}

type linkRequest struct {
	TransactionID int64 `json:"transaction_id"`
}

func (h *Handler) sessionDTO(s *domain.ReceiptSession) sessionDTO {
	if s == nil {
		return sessionDTO{}
	}
	return sessionDTO{
		Configured:       true,
		Phone:            s.Phone,
		DeviceID:         s.DeviceID,
		TokenExpiresAt:   h.fmtTime(s.TokenExpiresAt),
		RefreshExpiresAt: h.fmtTime(s.RefreshExpiresAt),
		UpdatedAt:        h.fmtTime(s.UpdatedAt),
	}
}

func (h *Handler) syncStatusDTO(st domain.ReceiptSyncStatus) syncStatusDTO {
	return syncStatusDTO{
		Running:    st.Running,
		StartedAt:  h.fmtTime(st.StartedAt),
		FinishedAt: h.fmtTime(st.FinishedAt),
		Listed:     st.Listed,
		Added:      st.Added,
		Detailed:   st.Detailed,
		Pending:    st.Pending,
		Matched:    st.Matched,
		Error:      st.Error,
	}
}

func (h *Handler) receiptDTO(r domain.Receipt, withItems bool) receiptDTO {
	out := receiptDTO{
		ID:            r.ID,
		Key:           r.Key,
		SellerName:    r.SellerName,
		SellerINN:     r.SellerINN,
		RetailPlace:   r.RetailPlace,
		RetailAddress: r.RetailAddress,
		IssuedAt:      h.fmtTime(r.IssuedAt),
		IssuedOn:      fmtDate(domain.DateOf(r.IssuedAt)),
		ReceivedAt:    h.fmtTime(r.ReceivedAt),
		OperationType: r.OperationType,
		Total:         int64(r.Total),
		CashTotal:     int64(r.CashTotal),
		EcashTotal:    int64(r.EcashTotal),
		ItemsLoaded:   r.ItemsLoaded,
		ItemsError:    r.ItemsError,
		TransactionID: r.TransactionID,
		MatchKind:     string(r.MatchKind),
		ItemCount:     r.ItemCount,
	}
	if withItems {
		out.Items = make([]receiptItemDTO, 0, len(r.Items))
		for _, it := range r.Items {
			out.Items = append(out.Items, receiptItemDTO{
				Position:   it.Position,
				Name:       it.Name,
				Price:      int64(it.Price),
				Quantity:   it.Quantity,
				Sum:        int64(it.Sum),
				CategoryID: it.CategoryID,
			})
		}
	}
	return out
}

func (h *Handler) getReceiptSession(w http.ResponseWriter, r *http.Request) {
	s, err := h.svc.ReceiptSession(r.Context())
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, h.sessionDTO(s))
}

func (h *Handler) putReceiptSession(w http.ResponseWriter, r *http.Request) {
	var req sessionRequest
	if err := decodeJSON(r, &req); err != nil {
		h.writeError(w, r, err)
		return
	}
	s, err := h.svc.SetReceiptSession(r.Context(), domain.ReceiptSession{
		Phone:        req.Phone,
		DeviceID:     req.DeviceID,
		RefreshToken: req.RefreshToken,
		AccessToken:  req.AccessToken,
	})
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, h.sessionDTO(s))
}

func (h *Handler) deleteReceiptSession(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.ClearReceiptSession(r.Context()); err != nil {
		h.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) receiptSyncStatus(w http.ResponseWriter, r *http.Request) {
	st, err := h.svc.ReceiptSyncStatus(r.Context())
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, h.syncStatusDTO(st))
}

func (h *Handler) startReceiptSync(w http.ResponseWriter, r *http.Request) {
	st, err := h.svc.StartReceiptSync(r.Context())
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, h.syncStatusDTO(st))
}

func (h *Handler) matchReceipts(w http.ResponseWriter, r *http.Request) {
	n, err := h.svc.MatchReceipts(r.Context())
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"matched": n})
}

func (h *Handler) listReceipts(w http.ResponseWriter, r *http.Request) {
	var f domain.ReceiptFilter
	var err error
	if f.From, err = queryDate(r, "from"); err != nil {
		h.writeError(w, r, err)
		return
	}
	if f.To, err = queryDate(r, "to"); err != nil {
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
	f.Unmatched = queryBool(r, "unmatched")

	items, total, err := h.svc.ListReceipts(r.Context(), f)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	out := make([]receiptDTO, 0, len(items))
	for _, rc := range items {
		out = append(out, h.receiptDTO(rc, false))
	}
	writeJSON(w, http.StatusOK, listDTO[receiptDTO]{Items: out, Total: total})
}

func (h *Handler) getReceipt(w http.ResponseWriter, r *http.Request) {
	id, err := urlID(r)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	rc, err := h.svc.GetReceipt(r.Context(), id)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, h.receiptDTO(*rc, true))
}

func (h *Handler) receiptCandidates(w http.ResponseWriter, r *http.Request) {
	id, err := urlID(r)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	items, err := h.svc.ReceiptCandidates(r.Context(), id)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	out := make([]transactionDTO, 0, len(items))
	for _, t := range items {
		out = append(out, h.transactionDTO(t))
	}
	writeJSON(w, http.StatusOK, listDTO[transactionDTO]{Items: out, Total: len(out)})
}

func (h *Handler) linkReceipt(w http.ResponseWriter, r *http.Request) {
	id, err := urlID(r)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	var req linkRequest
	if err := decodeJSON(r, &req); err != nil {
		h.writeError(w, r, err)
		return
	}
	if req.TransactionID <= 0 {
		h.badRequest(w, r, "transaction_id is required")
		return
	}
	rc, err := h.svc.LinkReceipt(r.Context(), id, req.TransactionID)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, h.receiptDTO(*rc, true))
}

func (h *Handler) unlinkReceipt(w http.ResponseWriter, r *http.Request) {
	id, err := urlID(r)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	if err := h.svc.UnlinkReceipt(r.Context(), id); err != nil {
		h.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) transactionReceipts(w http.ResponseWriter, r *http.Request) {
	id, err := urlID(r)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	list, err := h.svc.ReceiptsForTransaction(r.Context(), id)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	out := make([]receiptDTO, 0, len(list))
	for _, rc := range list {
		out = append(out, h.receiptDTO(rc, true))
	}
	writeJSON(w, http.StatusOK, listDTO[receiptDTO]{Items: out, Total: len(out)})
}

func (h *Handler) topItems(w http.ResponseWriter, r *http.Request) {
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
	items, err := h.svc.TopItems(r.Context(), q, limit)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	out := make([]itemTotalDTO, 0, len(items))
	for _, it := range items {
		out = append(out, itemTotalDTO{Name: it.Name, Quantity: it.Quantity, Sum: int64(it.Sum), Count: it.Count})
	}
	writeJSON(w, http.StatusOK, listDTO[itemTotalDTO]{Items: out, Total: len(out)})
}
