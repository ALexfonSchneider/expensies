-- A receipt without a bank line becomes a transaction of its own so the
-- analytics see spending the statement does not cover (other cards, days
-- after the last statement). Such rows have no statement.
ALTER TABLE transactions ALTER COLUMN statement_id DROP NOT NULL;
ALTER TABLE transactions ADD COLUMN source TEXT NOT NULL DEFAULT 'statement'
    CHECK (source IN ('statement', 'receipt'));
CREATE INDEX transactions_source_idx ON transactions (source);

ALTER TABLE receipts DROP CONSTRAINT receipts_match_kind_check;
ALTER TABLE receipts ADD CONSTRAINT receipts_match_kind_check
    CHECK (match_kind IN ('', 'auto', 'manual', 'virtual'));
