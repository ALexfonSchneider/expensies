DROP INDEX IF EXISTS receipts_transaction_id_idx;
-- Restoring the constraint fails if a transaction already owns several
-- receipts; detach the extras first.
ALTER TABLE receipts ADD CONSTRAINT receipts_transaction_key UNIQUE (transaction_id);
