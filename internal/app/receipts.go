package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ALexfonSchneider/expenses/internal/domain"
)

const (
	receiptPageSize     = 100
	receiptDetailsBatch = 500
	// resyncOverlap re-lists the last days on every run because the archive
	// delivers some receipts with a delay.
	resyncOverlap = 7 * 24 * time.Hour
	// matchTightWindow accepts the nearest candidate even among several;
	// matchLooseWindow accepts a lone candidate up to a day and a half away,
	// which covers deliveries that fiscalize on hand-over.
	matchTightWindow = 15 * time.Minute
	matchLooseWindow = 36 * time.Hour
	candidateWindow  = 3 * 24 * time.Hour
	defaultItemsTop  = 20
	maxItemsTop      = 200
)

// ReceiptSession returns the stored archive session, or nil.
func (s *Service) ReceiptSession(ctx context.Context) (*domain.ReceiptSession, error) {
	sess, err := s.sessions.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("app: load receipt session: %w", err)
	}
	return sess, nil
}

// SetReceiptSession stores credentials pasted from a browser session and
// checks them with one request. The access token is optional: a zero
// expiry makes the client refresh it right away.
func (s *Service) SetReceiptSession(ctx context.Context, in domain.ReceiptSession) (*domain.ReceiptSession, error) {
	in = expandPastedSession(in)
	phone, err := normalizePhone(in.Phone)
	if err != nil {
		return nil, err
	}
	refresh := strings.TrimSpace(in.RefreshToken)
	if refresh == "" {
		return nil, fmt.Errorf("%w: refresh token is required", domain.ErrInvalid)
	}
	device := strings.TrimSpace(in.DeviceID)
	if device == "" {
		device, err = randomID()
		if err != nil {
			return nil, fmt.Errorf("app: device id: %w", err)
		}
	}
	sess := &domain.ReceiptSession{
		Phone:        phone,
		DeviceID:     device,
		AccessToken:  strings.TrimSpace(in.AccessToken),
		RefreshToken: refresh,
		UpdatedAt:    s.now(),
	}
	if err := s.sessions.Save(ctx, sess); err != nil {
		return nil, fmt.Errorf("app: save receipt session: %w", err)
	}
	if err := s.receiptSource.Probe(ctx); err != nil {
		// A rejected token is useless; dropping it keeps the UI honest about
		// the archive being disconnected.
		if clearErr := s.sessions.Clear(ctx); clearErr != nil {
			s.logger.WarnContext(ctx, "clear rejected receipt session", "error", clearErr)
		}
		return nil, fmt.Errorf("%w: session check failed: %v", domain.ErrInvalid, err)
	}
	return s.ReceiptSession(ctx)
}

// ClearReceiptSession forgets the archive credentials.
func (s *Service) ClearReceiptSession(ctx context.Context) error {
	if err := s.sessions.Clear(ctx); err != nil {
		return fmt.Errorf("app: clear receipt session: %w", err)
	}
	return nil
}

// ReceiptSyncStatus reports the running or the last synchronization.
func (s *Service) ReceiptSyncStatus(ctx context.Context) (domain.ReceiptSyncStatus, error) {
	s.sync.mu.Lock()
	defer s.sync.mu.Unlock()
	if s.sync.status.Running || s.sync.loaded {
		return s.sync.status, nil
	}
	st, err := s.syncStore.LoadSyncStatus(ctx)
	if err != nil {
		return domain.ReceiptSyncStatus{}, fmt.Errorf("app: load sync status: %w", err)
	}
	if st != nil {
		s.sync.status = *st
	}
	s.sync.loaded = true
	return s.sync.status, nil
}

// StartReceiptSync launches a background synchronization. The archive
// throttles clients to about twenty requests a minute, so a first run over
// a long history takes a while; progress is visible through
// ReceiptSyncStatus and the run survives the HTTP request that started it.
func (s *Service) StartReceiptSync(ctx context.Context) (domain.ReceiptSyncStatus, error) {
	sess, err := s.ReceiptSession(ctx)
	if err != nil {
		return domain.ReceiptSyncStatus{}, err
	}
	if sess == nil {
		return domain.ReceiptSyncStatus{}, fmt.Errorf("app: start sync: %w", domain.ErrReceiptSession)
	}

	s.sync.mu.Lock()
	if s.sync.status.Running {
		st := s.sync.status
		s.sync.mu.Unlock()
		return st, fmt.Errorf("%w: sync is already running", domain.ErrAlreadyExists)
	}
	runCtx, cancel := context.WithCancel(s.bgCtx)
	s.sync.status = domain.ReceiptSyncStatus{Running: true, StartedAt: s.now()}
	s.sync.loaded = true
	s.sync.cancel = cancel
	s.sync.done = make(chan struct{})
	status := s.sync.status
	done := s.sync.done
	s.sync.mu.Unlock()

	go func() {
		defer close(done)
		s.runReceiptSync(runCtx)
	}()
	return status, nil
}

func (s *Service) runReceiptSync(ctx context.Context) {
	update := func(fn func(st *domain.ReceiptSyncStatus)) {
		s.sync.mu.Lock()
		fn(&s.sync.status)
		s.sync.mu.Unlock()
	}

	err := s.syncReceipts(ctx, update)
	now := s.now()
	update(func(st *domain.ReceiptSyncStatus) {
		st.Running = false
		st.FinishedAt = now
		if err != nil {
			st.Error = err.Error()
		}
	})
	s.sync.mu.Lock()
	final := s.sync.status
	s.sync.mu.Unlock()

	// The run context may already be cancelled by shutdown; the outcome is
	// still worth a few seconds to persist.
	saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := s.syncStore.SaveSyncStatus(saveCtx, &final, now); err != nil {
		s.logger.ErrorContext(saveCtx, "save receipt sync status", "error", err)
	}
	if err != nil {
		s.logger.WarnContext(saveCtx, "receipt sync finished with error", "error", err,
			"listed", final.Listed, "added", final.Added, "detailed", final.Detailed)
		return
	}
	s.logger.InfoContext(saveCtx, "receipt sync finished",
		"listed", final.Listed, "added", final.Added, "detailed", final.Detailed,
		"pending", final.Pending, "matched", final.Matched)
}

func (s *Service) syncReceipts(ctx context.Context, update func(func(*domain.ReceiptSyncStatus))) error {
	now := s.now()

	var since *time.Time
	latest, ok, err := s.receipts.LatestReceivedAt(ctx)
	if err != nil {
		return err
	}
	if ok {
		t := latest.Add(-resyncOverlap)
		since = &t
	}

	offset := 0
	for {
		page, more, err := s.receiptSource.ListReceipts(ctx, since, offset, receiptPageSize)
		if err != nil {
			return fmt.Errorf("list receipts: %w", err)
		}
		for i := range page {
			created, err := s.receipts.UpsertHeader(ctx, &page[i], now)
			if err != nil {
				return err
			}
			update(func(st *domain.ReceiptSyncStatus) {
				st.Listed++
				if created {
					st.Added++
				}
			})
		}
		if !more || len(page) == 0 {
			break
		}
		offset += len(page)
	}

	// Headers already carry the amount and the time, so links appear
	// before the slow per-receipt details phase.
	matched, err := s.MatchReceipts(ctx)
	if err != nil {
		return err
	}
	update(func(st *domain.ReceiptSyncStatus) {
		st.Matched += matched
	})

	pending, err := s.receipts.PendingDetails(ctx, receiptDetailsBatch)
	if err != nil {
		return err
	}
	update(func(st *domain.ReceiptSyncStatus) {
		st.Pending = len(pending)
	})
	for i := range pending {
		if err := ctx.Err(); err != nil {
			return err
		}
		r := &pending[i]
		err := s.receiptSource.LoadDetails(ctx, r)
		switch {
		case errors.Is(err, domain.ErrReceiptDetailsUnavailable):
			if err := s.receipts.SetItemsError(ctx, r.ID, "details not available"); err != nil {
				return err
			}
			update(func(st *domain.ReceiptSyncStatus) {
				st.Pending--
			})
		case errors.Is(err, domain.ErrReceiptSession):
			return err
		case err != nil:
			if ctx.Err() != nil {
				return ctx.Err()
			}
			// One flaky receipt must not abort the run; it stays pending.
			s.logger.WarnContext(ctx, "receipt details failed", "key", r.Key, "error", err)
		default:
			if err := s.receipts.SaveDetails(ctx, r); err != nil {
				return err
			}
			update(func(st *domain.ReceiptSyncStatus) {
				st.Detailed++
				st.Pending--
			})
		}
	}

	// Details can correct the fiscal time, so match once more at the end.
	matched, err = s.MatchReceipts(ctx)
	update(func(st *domain.ReceiptSyncStatus) {
		st.Matched += matched
	})
	return err
}

// MatchReceipts links unmatched receipts to transactions of the same
// amount when the timing makes the pair unambiguous: first one receipt to
// one transaction, then groups of receipts from one seller issued within
// minutes of each other whose totals add up to a single payment, which is
// how deliveries split goods, delivery and service fees.
func (s *Service) MatchReceipts(ctx context.Context) (int, error) {
	unmatched, err := s.receipts.Unmatched(ctx)
	if err != nil {
		return 0, fmt.Errorf("app: unmatched receipts: %w", err)
	}
	linked := 0
	rest := make([]domain.Receipt, 0, len(unmatched))
	for _, r := range unmatched {
		if r.Total <= 0 {
			continue
		}
		n, err := s.linkReceipts(ctx, []domain.Receipt{r}, r.Total, r.Direction(), r.IssuedAt)
		if err != nil {
			return linked, err
		}
		if n == 0 {
			rest = append(rest, r)
		}
		linked += n
	}
	for _, group := range receiptGroups(rest) {
		var total domain.Money
		for _, r := range group {
			total += r.Total
		}
		n, err := s.linkReceipts(ctx, group, total, group[0].Direction(), group[0].IssuedAt)
		if err != nil {
			return linked, err
		}
		linked += n
	}
	if err := s.materializeReceipts(ctx); err != nil {
		return linked, err
	}
	return linked, nil
}

// linkReceipts finds the transaction for the amount and attaches every
// receipt of the group to it. Returns how many receipts got linked.
func (s *Service) linkReceipts(ctx context.Context, group []domain.Receipt, total domain.Money, dir domain.Direction, issuedAt time.Time) (int, error) {
	cands, err := s.receipts.Candidates(ctx, total, dir, issuedAt.Add(-matchLooseWindow), issuedAt.Add(matchLooseWindow))
	if err != nil {
		return 0, fmt.Errorf("app: receipt candidates: %w", err)
	}
	txID, ok := chooseCandidate(issuedAt, cands)
	if !ok {
		return 0, nil
	}
	linked := 0
	for _, r := range group {
		if err := s.attachToStatement(ctx, r.ID, txID, domain.ReceiptMatchAuto); err != nil {
			if errors.Is(err, domain.ErrAlreadyExists) {
				continue
			}
			return linked, err
		}
		linked++
	}
	return linked, nil
}

// attachToStatement links a receipt to a bank line, first retiring the
// virtual transaction that stood in for it and carrying the edits made on
// that stand-in over to the bank line.
func (s *Service) attachToStatement(ctx context.Context, receiptID, txID int64, kind domain.ReceiptMatchKind) error {
	dropped, err := s.receipts.DropVirtual(ctx, receiptID)
	if err != nil {
		return fmt.Errorf("app: drop virtual transaction: %w", err)
	}
	if err := s.receipts.Link(ctx, receiptID, txID, kind); err != nil {
		return fmt.Errorf("app: link receipt: %w", err)
	}
	if dropped == nil {
		return nil
	}
	patch := domain.TransactionPatch{}
	if dropped.CategorySource == domain.CategorySourceManual {
		patch.SetCategory = true
		patch.CategoryID = dropped.CategoryID
	}
	if dropped.Note != "" {
		note := dropped.Note
		patch.Note = &note
	}
	if dropped.Excluded {
		excluded := true
		patch.Excluded = &excluded
	}
	if !patch.SetCategory && patch.Note == nil && patch.Excluded == nil {
		return nil
	}
	if err := s.transactions.Update(ctx, txID, patch); err != nil {
		return fmt.Errorf("app: carry over receipt edits: %w", err)
	}
	return nil
}

// materializeReceipts turns every receipt that no statement covers into a
// transaction of its own, so spending from other cards and from days after
// the last statement is part of the picture. Only documents that moved
// money qualify; a stand-in created earlier for a receipt that no longer
// qualifies (details arrived and showed a zero payment, a refund twin
// appeared) is removed again.
func (s *Service) materializeReceipts(ctx context.Context) error {
	unmatched, err := s.receipts.Unmatched(ctx)
	if err != nil {
		return fmt.Errorf("app: unmatched receipts: %w", err)
	}
	reversed := reversedReceipts(unmatched)
	var cat *Categorizer
	now := s.now()
	for _, r := range unmatched {
		if !standsAlone(r, reversed) {
			if r.MatchKind == domain.ReceiptMatchVirtual {
				if _, err := s.receipts.DropVirtual(ctx, r.ID); err != nil {
					return fmt.Errorf("app: drop virtual transaction: %w", err)
				}
			}
			continue
		}
		if r.TransactionID != nil {
			continue
		}
		if cat == nil {
			if cat, err = s.categorizer(ctx); err != nil {
				return err
			}
		}
		tx := transactionFromReceipt(r)
		if id, ok := cat.Match(&tx); ok {
			tx.CategoryID = &id
			tx.CategorySource = domain.CategorySourceRule
		}
		if err := s.receipts.AttachVirtual(ctx, r.ID, &tx, now); err != nil {
			if errors.Is(err, domain.ErrAlreadyExists) {
				continue
			}
			return fmt.Errorf("app: materialize receipt: %w", err)
		}
	}
	return nil
}

// standsAlone says whether a receipt without a bank line represents real
// spending or income on its own.
func standsAlone(r domain.Receipt, reversed map[int64]bool) bool {
	if r.Total <= 0 || !r.IsPayment() || reversed[r.ID] {
		return false
	}
	return r.OperationType == domain.ReceiptOpSale || r.OperationType == domain.ReceiptOpSaleRefund
}

// reversalWindow is how soon a cancelled order gets its refund receipt.
const reversalWindow = 24 * time.Hour

// reversedReceipts finds sale receipts undone by a refund of the same
// amount from the same seller shortly after: a cancelled order where the
// bank never posted the charge. Both documents are reported.
func reversedReceipts(receipts []domain.Receipt) map[int64]bool {
	out := make(map[int64]bool)
	type key struct {
		inn   string
		total domain.Money
	}
	sales := make(map[key][]domain.Receipt)
	for _, r := range receipts {
		if r.OperationType == domain.ReceiptOpSale && r.SellerINN != "" {
			k := key{r.SellerINN, r.Total}
			sales[k] = append(sales[k], r)
		}
	}
	for _, r := range receipts {
		if r.OperationType != domain.ReceiptOpSaleRefund || r.SellerINN == "" {
			continue
		}
		k := key{r.SellerINN, r.Total}
		for _, sale := range sales[k] {
			if out[sale.ID] {
				continue
			}
			d := r.IssuedAt.Sub(sale.IssuedAt)
			if d < 0 {
				d = -d
			}
			if d <= reversalWindow {
				out[sale.ID] = true
				out[r.ID] = true
				break
			}
		}
	}
	return out
}

// transactionFromReceipt builds the stand-in line for a receipt.
func transactionFromReceipt(r domain.Receipt) domain.Transaction {
	merchant := r.RetailPlace
	if merchant == "" {
		merchant = ShortLegalName(r.SellerName)
	}
	kind := domain.KindPurchase
	if r.OperationType == domain.ReceiptOpSaleRefund {
		kind = domain.KindRefund
	}
	desc := "Чек " + ShortLegalName(r.SellerName)
	if r.RetailPlace != "" && r.RetailPlace != r.SellerName {
		desc += " (" + r.RetailPlace + ")"
	}
	return domain.Transaction{
		Source:      domain.SourceReceipt,
		Fingerprint: "receipt:" + r.Source + ":" + r.Key,
		OpAt:        r.IssuedAt,
		ProcessedOn: domain.DateOf(r.IssuedAt),
		Amount:      r.Total,
		Direction:   r.Direction(),
		Kind:        kind,
		Description: strings.TrimSpace(desc),
		Merchant:    strings.TrimSpace(merchant),
		Counterparty: func() string {
			if r.SellerINN != "" {
				return "ИНН " + r.SellerINN
			}
			return ""
		}(),
	}
}

// legalForms are the long forms printed on receipts and their usual
// abbreviations.
var legalForms = []struct{ long, short string }{
	{"ОБЩЕСТВО С ОГРАНИЧЕННОЙ ОТВЕТСТВЕННОСТЬЮ", "ООО"},
	{"ПУБЛИЧНОЕ АКЦИОНЕРНОЕ ОБЩЕСТВО", "ПАО"},
	{"АКЦИОНЕРНОЕ ОБЩЕСТВО", "АО"},
	{"ИНДИВИДУАЛЬНЫЙ ПРЕДПРИНИМАТЕЛЬ", "ИП"},
}

// ShortLegalName abbreviates the legal form at the start of a seller name.
func ShortLegalName(name string) string {
	trimmed := strings.TrimSpace(name)
	upper := strings.ToUpper(trimmed)
	for _, f := range legalForms {
		if strings.HasPrefix(upper, f.long) {
			return f.short + " " + strings.TrimSpace(trimmed[len(f.long):])
		}
	}
	return trimmed
}

// groupWindow is how far apart the documents of one split payment are
// issued; a delivery prints them within the same minute or two.
const groupWindow = 10 * time.Minute

// receiptGroups clusters receipts of one seller and direction issued within
// groupWindow of each other. Single receipts are dropped: they were already
// tried on their own.
func receiptGroups(receipts []domain.Receipt) [][]domain.Receipt {
	sorted := make([]domain.Receipt, len(receipts))
	copy(sorted, receipts)
	// Direction is part of the sort key so a refund issued between two
	// sale documents does not split their group.
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].SellerINN != sorted[j].SellerINN {
			return sorted[i].SellerINN < sorted[j].SellerINN
		}
		if di, dj := sorted[i].Direction(), sorted[j].Direction(); di != dj {
			return di < dj
		}
		return sorted[i].IssuedAt.Before(sorted[j].IssuedAt)
	})
	groups := make([][]domain.Receipt, 0)
	var cur []domain.Receipt
	flush := func() {
		if len(cur) > 1 {
			groups = append(groups, cur)
		}
		cur = nil
	}
	for _, r := range sorted {
		if r.SellerINN == "" {
			continue
		}
		if len(cur) > 0 {
			last := cur[len(cur)-1]
			if last.SellerINN != r.SellerINN || last.Direction() != r.Direction() || r.IssuedAt.Sub(cur[0].IssuedAt) > groupWindow {
				flush()
			}
		}
		cur = append(cur, r)
	}
	flush()
	return groups
}

// chooseCandidate picks the transaction closest in time to the receipt. A
// match inside the tight window wins even among several candidates;
// outside it the candidate must be the only one, because a second equal
// amount within a day and a half is a coin toss.
func chooseCandidate(issuedAt time.Time, cands []domain.Transaction) (int64, bool) {
	if len(cands) == 0 {
		return 0, false
	}
	best := -1
	var bestDiff time.Duration
	for i, c := range cands {
		d := c.OpAt.Sub(issuedAt)
		if d < 0 {
			d = -d
		}
		if best < 0 || d < bestDiff {
			best, bestDiff = i, d
		}
	}
	if bestDiff <= matchTightWindow {
		return cands[best].ID, true
	}
	if len(cands) == 1 && bestDiff <= matchLooseWindow {
		return cands[0].ID, true
	}
	return 0, false
}

// ListReceipts returns a page of receipts.
func (s *Service) ListReceipts(ctx context.Context, f domain.ReceiptFilter) ([]domain.Receipt, int, error) {
	if f.Limit <= 0 {
		f.Limit = defaultPageSize
	}
	if f.Limit > maxPageSize {
		f.Limit = maxPageSize
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
	items, total, err := s.receipts.List(ctx, f)
	if err != nil {
		return nil, 0, fmt.Errorf("app: list receipts: %w", err)
	}
	return items, total, nil
}

// GetReceipt returns a receipt with its items.
func (s *Service) GetReceipt(ctx context.Context, id int64) (*domain.Receipt, error) {
	r, err := s.receipts.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("app: get receipt: %w", err)
	}
	return r, nil
}

// ReceiptsForTransaction returns the receipts linked to a transaction.
func (s *Service) ReceiptsForTransaction(ctx context.Context, txID int64) ([]domain.Receipt, error) {
	list, err := s.receipts.ByTransaction(ctx, txID)
	if err != nil {
		return nil, fmt.Errorf("app: receipts for transaction: %w", err)
	}
	return list, nil
}

// ReceiptCandidates lists unlinked transactions a receipt could belong to.
func (s *Service) ReceiptCandidates(ctx context.Context, id int64) ([]domain.Transaction, error) {
	r, err := s.receipts.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("app: get receipt: %w", err)
	}
	cands, err := s.receipts.Candidates(ctx, r.Total, r.Direction(),
		r.IssuedAt.Add(-candidateWindow), r.IssuedAt.Add(candidateWindow))
	if err != nil {
		return nil, fmt.Errorf("app: receipt candidates: %w", err)
	}
	return cands, nil
}

// LinkReceipt attaches a receipt to a transaction by hand.
func (s *Service) LinkReceipt(ctx context.Context, id, txID int64) (*domain.Receipt, error) {
	if _, err := s.transactions.Get(ctx, txID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, fmt.Errorf("%w: transaction %d does not exist", domain.ErrInvalid, txID)
		}
		return nil, fmt.Errorf("app: check transaction: %w", err)
	}
	if err := s.attachToStatement(ctx, id, txID, domain.ReceiptMatchManual); err != nil {
		return nil, err
	}
	return s.GetReceipt(ctx, id)
}

// UnlinkReceipt detaches a receipt from its bank line; the receipt then
// gets a stand-in transaction again on the next matching pass.
func (s *Service) UnlinkReceipt(ctx context.Context, id int64) error {
	if err := s.receipts.Unlink(ctx, id); err != nil {
		return fmt.Errorf("app: unlink receipt: %w", err)
	}
	return s.materializeReceipts(ctx)
}

// TopItems returns the most expensive product names over a period.
func (s *Service) TopItems(ctx context.Context, q domain.RangeQuery, limit int) ([]domain.ItemTotal, error) {
	if err := validateRange(q); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = defaultItemsTop
	}
	if limit > maxItemsTop {
		limit = maxItemsTop
	}
	items, err := s.receipts.TopItems(ctx, q, limit)
	if err != nil {
		return nil, fmt.Errorf("app: top items: %w", err)
	}
	return items, nil
}

// normalizePhone reduces a Russian phone number to eleven digits starting
// with 7, which is the form the archive expects.
func normalizePhone(s string) (string, error) {
	var digits strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	d := digits.String()
	switch {
	case len(d) == 11 && d[0] == '8':
		d = "7" + d[1:]
	case len(d) == 10:
		d = "7" + d
	}
	if len(d) != 11 || d[0] != '7' {
		return "", fmt.Errorf("%w: phone must be a Russian number like +7 999 123-45-67", domain.ErrInvalid)
	}
	return d, nil
}

func randomID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
