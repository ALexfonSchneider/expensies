-- Deliveries fiscalize one payment as several documents (goods, delivery
-- fee, service fee), so a transaction may own many receipts.
ALTER TABLE receipts DROP CONSTRAINT IF EXISTS receipts_transaction_key;
CREATE INDEX IF NOT EXISTS receipts_transaction_id_idx ON receipts (transaction_id);
