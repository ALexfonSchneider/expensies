package postgresrepo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/ALexfonSchneider/expenses/internal/domain"
)

const receiptColumns = `r.id, r.source, r.key, r.fiscal_drive_number, r.fiscal_document_number, r.fiscal_sign,
	r.seller_name, r.seller_inn, r.retail_place, r.retail_address, r.issued_at, r.received_at, r.operation_type,
	r.total, r.cash_total, r.ecash_total, r.items_loaded, r.items_error, r.transaction_id, r.match_kind,
	(SELECT count(*) FROM receipt_items i WHERE i.receipt_id = r.id)`

func scanReceipt(s scanner) (domain.Receipt, error) {
	var r domain.Receipt
	var total, cash, ecash int64
	var kind string
	err := s.Scan(&r.ID, &r.Source, &r.Key, &r.FiscalDriveNumber, &r.FiscalDocumentNumber, &r.FiscalSign,
		&r.SellerName, &r.SellerINN, &r.RetailPlace, &r.RetailAddress, &r.IssuedAt, &r.ReceivedAt, &r.OperationType,
		&total, &cash, &ecash, &r.ItemsLoaded, &r.ItemsError, &r.TransactionID, &kind, &r.ItemCount)
	if err != nil {
		return domain.Receipt{}, err
	}
	r.Total = domain.Money(total)
	r.CashTotal = domain.Money(cash)
	r.EcashTotal = domain.Money(ecash)
	r.MatchKind = domain.ReceiptMatchKind(kind)
	return r, nil
}

func collectReceipts(rows pgx.Rows) ([]domain.Receipt, error) {
	defer rows.Close()
	out := make([]domain.Receipt, 0)
	for rows.Next() {
		r, err := scanReceipt(rows)
		if err != nil {
			return nil, fmt.Errorf("postgresrepo: scan receipt: %w", err)
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgresrepo: read receipts: %w", err)
	}
	return out, nil
}

// UpsertHeader implements domain.ReceiptRepository. xmax = 0 tells an
// insert from an update on the conflict path.
func (r *Receipts) UpsertHeader(ctx context.Context, rc *domain.Receipt, now time.Time) (bool, error) {
	var inserted bool
	err := r.q().QueryRow(ctx, `
		INSERT INTO receipts (source, key, fiscal_drive_number, fiscal_document_number, seller_name, seller_inn,
		                      issued_at, issued_on, received_at, total, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (source, key) DO UPDATE SET received_at = EXCLUDED.received_at
		RETURNING id, items_loaded, (xmax = 0)`,
		rc.Source, rc.Key, rc.FiscalDriveNumber, rc.FiscalDocumentNumber, rc.SellerName, rc.SellerINN,
		rc.IssuedAt, domain.DateOf(rc.IssuedAt), rc.ReceivedAt, int64(rc.Total), now,
	).Scan(&rc.ID, &rc.ItemsLoaded, &inserted)
	if err != nil {
		return false, fmt.Errorf("postgresrepo: upsert receipt %s: %w", rc.Key, err)
	}
	return inserted, nil
}

// SaveDetails implements domain.ReceiptRepository.
func (r *Receipts) SaveDetails(ctx context.Context, rc *domain.Receipt) error {
	return r.db.WithTx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE receipts SET fiscal_drive_number = $2, fiscal_document_number = $3, fiscal_sign = $4,
			       seller_name = $5, seller_inn = $6, retail_place = $7, retail_address = $8,
			       issued_at = $9, issued_on = $10, operation_type = $11, total = $12, cash_total = $13, ecash_total = $14,
			       items_loaded = TRUE, items_error = ''
			WHERE id = $1`,
			rc.ID, rc.FiscalDriveNumber, rc.FiscalDocumentNumber, rc.FiscalSign,
			rc.SellerName, rc.SellerINN, rc.RetailPlace, rc.RetailAddress,
			rc.IssuedAt, domain.DateOf(rc.IssuedAt), rc.OperationType, int64(rc.Total), int64(rc.CashTotal), int64(rc.EcashTotal))
		if err != nil {
			return fmt.Errorf("postgresrepo: update receipt: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("postgresrepo: receipt %d: %w", rc.ID, domain.ErrNotFound)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM receipt_items WHERE receipt_id = $1`, rc.ID); err != nil {
			return fmt.Errorf("postgresrepo: clear items: %w", err)
		}
		if len(rc.Items) == 0 {
			return nil
		}
		batch := &pgx.Batch{}
		for _, it := range rc.Items {
			batch.Queue(`
				INSERT INTO receipt_items (receipt_id, position, name, price, quantity, sum, product_type, category_id)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
				rc.ID, it.Position, it.Name, int64(it.Price), it.Quantity, int64(it.Sum), it.ProductType, it.CategoryID)
		}
		results := tx.SendBatch(ctx, batch)
		for range rc.Items {
			if _, err := results.Exec(); err != nil {
				_ = results.Close() // the batch already failed; WithTx rolls back
				return fmt.Errorf("postgresrepo: insert items: %w", err)
			}
		}
		if err := results.Close(); err != nil {
			return fmt.Errorf("postgresrepo: insert items: %w", err)
		}
		return nil
	})
}

// SetItemsError implements domain.ReceiptRepository.
func (r *Receipts) SetItemsError(ctx context.Context, id int64, msg string) error {
	if _, err := r.q().Exec(ctx, `UPDATE receipts SET items_error = $2 WHERE id = $1`, id, msg); err != nil {
		return fmt.Errorf("postgresrepo: set items error: %w", err)
	}
	return nil
}

// PendingDetails implements domain.ReceiptRepository.
func (r *Receipts) PendingDetails(ctx context.Context, limit int) ([]domain.Receipt, error) {
	rows, err := r.q().Query(ctx, `
		SELECT `+receiptColumns+` FROM receipts r
		WHERE NOT r.items_loaded AND r.items_error = ''
		ORDER BY r.received_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("postgresrepo: pending receipts: %w", err)
	}
	return collectReceipts(rows)
}

// List implements domain.ReceiptRepository.
func (r *Receipts) List(ctx context.Context, f domain.ReceiptFilter) ([]domain.Receipt, int, error) {
	w := &whereBuilder{}
	if f.From != nil {
		w.add("r.issued_on >= $%d", *f.From)
	}
	if f.To != nil {
		w.add("r.issued_on <= $%d", *f.To)
	}
	if f.Unmatched {
		w.raw("(r.transaction_id IS NULL OR r.match_kind = 'virtual')")
	}
	from := ` FROM receipts r` + w.sql()

	var total int
	if err := r.q().QueryRow(ctx, `SELECT count(*)`+from, w.args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("postgresrepo: count receipts: %w", err)
	}
	args := append(append([]any{}, w.args...), f.Limit, f.Offset)
	rows, err := r.q().Query(ctx,
		`SELECT `+receiptColumns+from+fmt.Sprintf(` ORDER BY r.issued_at DESC, r.id DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args)),
		args...)
	if err != nil {
		return nil, 0, fmt.Errorf("postgresrepo: list receipts: %w", err)
	}
	items, err := collectReceipts(rows)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *Receipts) loadItems(ctx context.Context, rc *domain.Receipt) error {
	rows, err := r.q().Query(ctx, `
		SELECT id, receipt_id, position, name, price, quantity, sum, product_type, category_id
		FROM receipt_items WHERE receipt_id = $1 ORDER BY position`, rc.ID)
	if err != nil {
		return fmt.Errorf("postgresrepo: receipt items: %w", err)
	}
	defer rows.Close()
	rc.Items = make([]domain.ReceiptItem, 0)
	for rows.Next() {
		var it domain.ReceiptItem
		var price, sum int64
		if err := rows.Scan(&it.ID, &it.ReceiptID, &it.Position, &it.Name, &price, &it.Quantity, &sum, &it.ProductType, &it.CategoryID); err != nil {
			return fmt.Errorf("postgresrepo: scan receipt item: %w", err)
		}
		it.Price = domain.Money(price)
		it.Sum = domain.Money(sum)
		rc.Items = append(rc.Items, it)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("postgresrepo: receipt items: %w", err)
	}
	rc.ItemCount = len(rc.Items)
	return nil
}

// Get implements domain.ReceiptRepository.
func (r *Receipts) Get(ctx context.Context, id int64) (*domain.Receipt, error) {
	rc, err := scanReceipt(r.q().QueryRow(ctx, `SELECT `+receiptColumns+` FROM receipts r WHERE r.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("postgresrepo: receipt %d: %w", id, domain.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("postgresrepo: get receipt: %w", err)
	}
	if err := r.loadItems(ctx, &rc); err != nil {
		return nil, err
	}
	return &rc, nil
}

// ByTransaction implements domain.ReceiptRepository.
func (r *Receipts) ByTransaction(ctx context.Context, txID int64) ([]domain.Receipt, error) {
	rows, err := r.q().Query(ctx,
		`SELECT `+receiptColumns+` FROM receipts r WHERE r.transaction_id = $1 ORDER BY r.issued_at, r.id`, txID)
	if err != nil {
		return nil, fmt.Errorf("postgresrepo: receipts by transaction: %w", err)
	}
	list, err := collectReceipts(rows)
	if err != nil {
		return nil, err
	}
	for i := range list {
		if err := r.loadItems(ctx, &list[i]); err != nil {
			return nil, err
		}
	}
	return list, nil
}

// Unmatched implements domain.ReceiptRepository.
func (r *Receipts) Unmatched(ctx context.Context) ([]domain.Receipt, error) {
	rows, err := r.q().Query(ctx, `
		SELECT `+receiptColumns+` FROM receipts r
		WHERE r.transaction_id IS NULL OR r.match_kind = 'virtual'
		ORDER BY r.issued_at`)
	if err != nil {
		return nil, fmt.Errorf("postgresrepo: unmatched receipts: %w", err)
	}
	return collectReceipts(rows)
}

// Candidates implements domain.ReceiptRepository.
func (r *Receipts) Candidates(ctx context.Context, total domain.Money, dir domain.Direction, from, to time.Time) ([]domain.Transaction, error) {
	rows, err := r.q().Query(ctx, `
		SELECT `+txColumns+` FROM transactions t
		WHERE t.source = 'statement' AND t.amount = $1 AND t.direction = $2 AND t.op_at BETWEEN $3 AND $4
		  AND NOT EXISTS (SELECT 1 FROM receipts x WHERE x.transaction_id = t.id)
		ORDER BY t.op_at`,
		int64(total), string(dir), from, to)
	if err != nil {
		return nil, fmt.Errorf("postgresrepo: receipt candidates: %w", err)
	}
	return collectTransactions(rows)
}

// AttachVirtual implements domain.ReceiptRepository.
func (r *Receipts) AttachVirtual(ctx context.Context, receiptID int64, tx *domain.Transaction, now time.Time) error {
	return r.db.WithTx(ctx, func(dbtx pgx.Tx) error {
		err := dbtx.QueryRow(ctx, `
			INSERT INTO transactions (statement_id, source, fingerprint, op_at, op_date, processed_on, card, amount,
			                          direction, kind, description, merchant, mcc, counterparty,
			                          category_id, category_source, note, created_at)
			VALUES (NULL, $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
			RETURNING id`,
			string(domain.SourceReceipt), tx.Fingerprint, tx.OpAt, domain.DateOf(tx.OpAt), tx.ProcessedOn, tx.Card, int64(tx.Amount),
			string(tx.Direction), string(tx.Kind), tx.Description, tx.Merchant, tx.MCC, tx.Counterparty,
			tx.CategoryID, string(tx.CategorySource), tx.Note, now,
		).Scan(&tx.ID)
		if pgCode(err) == pgUniqueViolation {
			return fmt.Errorf("postgresrepo: virtual transaction for receipt %d: %w", receiptID, domain.ErrAlreadyExists)
		}
		if err != nil {
			return fmt.Errorf("postgresrepo: insert virtual transaction: %w", err)
		}
		tag, err := dbtx.Exec(ctx, `
			UPDATE receipts SET transaction_id = $2, match_kind = $3 WHERE id = $1 AND transaction_id IS NULL`,
			receiptID, tx.ID, string(domain.ReceiptMatchVirtual))
		if err != nil {
			return fmt.Errorf("postgresrepo: attach virtual transaction: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return fmt.Errorf("postgresrepo: receipt %d is not unmatched: %w", receiptID, domain.ErrAlreadyExists)
		}
		return nil
	})
}

// DropVirtual implements domain.ReceiptRepository.
func (r *Receipts) DropVirtual(ctx context.Context, receiptID int64) (*domain.Transaction, error) {
	var dropped *domain.Transaction
	err := r.db.WithTx(ctx, func(dbtx pgx.Tx) error {
		var txID *int64
		var kind string
		err := dbtx.QueryRow(ctx, `SELECT transaction_id, match_kind FROM receipts WHERE id = $1 FOR UPDATE`, receiptID).Scan(&txID, &kind)
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("postgresrepo: receipt %d: %w", receiptID, domain.ErrNotFound)
		}
		if err != nil {
			return fmt.Errorf("postgresrepo: read receipt link: %w", err)
		}
		if txID == nil || kind != string(domain.ReceiptMatchVirtual) {
			return nil
		}
		t, err := scanTransaction(dbtx.QueryRow(ctx, `SELECT `+txColumns+` FROM transactions t WHERE t.id = $1 AND t.source = 'receipt'`, *txID))
		if errors.Is(err, pgx.ErrNoRows) {
			// The link points at a row that is not virtual; leave it alone.
			return nil
		}
		if err != nil {
			return fmt.Errorf("postgresrepo: read virtual transaction: %w", err)
		}
		if _, err := dbtx.Exec(ctx, `UPDATE receipts SET transaction_id = NULL, match_kind = '' WHERE id = $1`, receiptID); err != nil {
			return fmt.Errorf("postgresrepo: detach virtual transaction: %w", err)
		}
		if _, err := dbtx.Exec(ctx, `DELETE FROM transactions WHERE id = $1 AND source = 'receipt'`, *txID); err != nil {
			return fmt.Errorf("postgresrepo: delete virtual transaction: %w", err)
		}
		dropped = &t
		return nil
	})
	if err != nil {
		return nil, err
	}
	return dropped, nil
}

// Link implements domain.ReceiptRepository.
func (r *Receipts) Link(ctx context.Context, id, txID int64, kind domain.ReceiptMatchKind) error {
	guard := ""
	if kind == domain.ReceiptMatchAuto {
		guard = " AND transaction_id IS NULL"
	}
	tag, err := r.q().Exec(ctx, `UPDATE receipts SET transaction_id = $2, match_kind = $3 WHERE id = $1`+guard,
		id, txID, string(kind))
	if pgCode(err) == pgForeignKeyViolation {
		return fmt.Errorf("postgresrepo: transaction %d does not exist: %w", txID, domain.ErrInvalid)
	}
	if err != nil {
		return fmt.Errorf("postgresrepo: link receipt: %w", err)
	}
	if tag.RowsAffected() == 0 {
		if kind == domain.ReceiptMatchAuto {
			return fmt.Errorf("postgresrepo: receipt %d is already linked: %w", id, domain.ErrAlreadyExists)
		}
		return fmt.Errorf("postgresrepo: receipt %d: %w", id, domain.ErrNotFound)
	}
	return nil
}

// Unlink implements domain.ReceiptRepository.
func (r *Receipts) Unlink(ctx context.Context, id int64) error {
	tag, err := r.q().Exec(ctx, `UPDATE receipts SET transaction_id = NULL, match_kind = '' WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("postgresrepo: unlink receipt: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("postgresrepo: receipt %d: %w", id, domain.ErrNotFound)
	}
	return nil
}

// LatestReceivedAt implements domain.ReceiptRepository.
func (r *Receipts) LatestReceivedAt(ctx context.Context) (time.Time, bool, error) {
	var latest *time.Time
	if err := r.q().QueryRow(ctx, `SELECT MAX(received_at) FROM receipts`).Scan(&latest); err != nil {
		return time.Time{}, false, fmt.Errorf("postgresrepo: latest receipt: %w", err)
	}
	if latest == nil {
		return time.Time{}, false, nil
	}
	return *latest, true, nil
}

// TopItems implements domain.ReceiptRepository. Names are grouped
// case-insensitively because shops print the same product both ways.
func (r *Receipts) TopItems(ctx context.Context, q domain.RangeQuery, limit int) ([]domain.ItemTotal, error) {
	rows, err := r.q().Query(ctx, `
		SELECT min(i.name), SUM(i.quantity), SUM(i.sum), COUNT(*)
		FROM receipt_items i JOIN receipts r ON r.id = i.receipt_id
		WHERE r.issued_on BETWEEN $1 AND $2 AND r.operation_type = $3
		GROUP BY lower(i.name)
		ORDER BY 3 DESC LIMIT $4`,
		q.From, q.To, domain.ReceiptOpSale, limit)
	if err != nil {
		return nil, fmt.Errorf("postgresrepo: top items: %w", err)
	}
	defer rows.Close()
	out := make([]domain.ItemTotal, 0)
	for rows.Next() {
		var it domain.ItemTotal
		var sum int64
		if err := rows.Scan(&it.Name, &it.Quantity, &sum, &it.Count); err != nil {
			return nil, fmt.Errorf("postgresrepo: scan item total: %w", err)
		}
		it.Sum = domain.Money(sum)
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgresrepo: top items: %w", err)
	}
	return out, nil
}
