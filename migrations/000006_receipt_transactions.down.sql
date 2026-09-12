DELETE FROM transactions WHERE source = 'receipt';
UPDATE receipts SET transaction_id = NULL, match_kind = '' WHERE match_kind = 'virtual';
ALTER TABLE receipts DROP CONSTRAINT receipts_match_kind_check;
ALTER TABLE receipts ADD CONSTRAINT receipts_match_kind_check
    CHECK (match_kind IN ('', 'auto', 'manual'));
DROP INDEX IF EXISTS transactions_source_idx;
ALTER TABLE transactions DROP COLUMN source;
ALTER TABLE transactions ALTER COLUMN statement_id SET NOT NULL;
