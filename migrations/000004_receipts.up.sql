CREATE TABLE settings (
    key        TEXT        PRIMARY KEY,
    value      JSONB       NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE receipts (
    id                     BIGSERIAL   PRIMARY KEY,
    source                 TEXT        NOT NULL DEFAULT 'lkdr',
    key                    TEXT        NOT NULL,
    fiscal_drive_number    TEXT        NOT NULL DEFAULT '',
    fiscal_document_number TEXT        NOT NULL DEFAULT '',
    fiscal_sign            TEXT        NOT NULL DEFAULT '',
    seller_name            TEXT        NOT NULL DEFAULT '',
    seller_inn             TEXT        NOT NULL DEFAULT '',
    retail_place           TEXT        NOT NULL DEFAULT '',
    retail_address         TEXT        NOT NULL DEFAULT '',
    issued_at              TIMESTAMPTZ NOT NULL,
    issued_on              DATE        NOT NULL,
    received_at            TIMESTAMPTZ NOT NULL,
    operation_type         INTEGER     NOT NULL DEFAULT 1,
    total                  BIGINT      NOT NULL,
    cash_total             BIGINT      NOT NULL DEFAULT 0,
    ecash_total            BIGINT      NOT NULL DEFAULT 0,
    items_loaded           BOOLEAN     NOT NULL DEFAULT FALSE,
    items_error            TEXT        NOT NULL DEFAULT '',
    transaction_id         BIGINT      REFERENCES transactions (id) ON DELETE SET NULL,
    match_kind             TEXT        NOT NULL DEFAULT '' CHECK (match_kind IN ('', 'auto', 'manual')),
    created_at             TIMESTAMPTZ NOT NULL,
    CONSTRAINT receipts_source_key UNIQUE (source, key),
    -- One receipt per operation; a receipt without a match keeps NULL.
    CONSTRAINT receipts_transaction_key UNIQUE (transaction_id)
);

CREATE INDEX receipts_issued_on_idx ON receipts (issued_on);
CREATE INDEX receipts_received_at_idx ON receipts (received_at);

CREATE TABLE receipt_items (
    id           BIGSERIAL      PRIMARY KEY,
    receipt_id   BIGINT         NOT NULL REFERENCES receipts (id) ON DELETE CASCADE,
    position     INTEGER        NOT NULL,
    name         TEXT           NOT NULL,
    price        BIGINT         NOT NULL,
    quantity     NUMERIC(14, 3) NOT NULL,
    sum          BIGINT         NOT NULL,
    product_type INTEGER        NOT NULL DEFAULT 0,
    category_id  BIGINT         REFERENCES categories (id) ON DELETE SET NULL
);

CREATE INDEX receipt_items_receipt_id_idx ON receipt_items (receipt_id);
CREATE INDEX receipt_items_name_idx ON receipt_items (lower(name));
