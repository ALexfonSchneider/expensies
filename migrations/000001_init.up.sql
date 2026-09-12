CREATE TABLE statements (
    id              BIGSERIAL   PRIMARY KEY,
    bank            TEXT        NOT NULL,
    number          TEXT        NOT NULL,
    period_from     DATE        NOT NULL,
    period_to       DATE        NOT NULL,
    opening_balance BIGINT      NOT NULL DEFAULT 0,
    closing_balance BIGINT      NOT NULL DEFAULT 0,
    total_income    BIGINT      NOT NULL DEFAULT 0,
    total_expense   BIGINT      NOT NULL DEFAULT 0,
    file_name       TEXT        NOT NULL,
    file_sha256     TEXT        NOT NULL,
    file_bytes      BYTEA       NOT NULL,
    tx_count        INTEGER     NOT NULL DEFAULT 0,
    uploaded_at     TIMESTAMPTZ NOT NULL,
    CONSTRAINT statements_bank_number_key UNIQUE (bank, number),
    CONSTRAINT statements_file_sha256_key UNIQUE (file_sha256)
);

CREATE TABLE categories (
    id          BIGSERIAL   PRIMARY KEY,
    name        TEXT        NOT NULL,
    color       TEXT        NOT NULL DEFAULT '#9e9e9e',
    is_transfer BOOLEAN     NOT NULL DEFAULT FALSE,
    sort_order  INTEGER     NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL,
    CONSTRAINT categories_name_key UNIQUE (name)
);

CREATE TABLE category_rules (
    id          BIGSERIAL   PRIMARY KEY,
    category_id BIGINT      NOT NULL REFERENCES categories (id) ON DELETE CASCADE,
    field       TEXT        NOT NULL CHECK (field IN ('merchant', 'description', 'mcc', 'kind', 'counterparty')),
    op          TEXT        NOT NULL CHECK (op IN ('contains', 'equals', 'prefix', 'regex')),
    value       TEXT        NOT NULL,
    priority    INTEGER     NOT NULL DEFAULT 100,
    created_at  TIMESTAMPTZ NOT NULL
);

CREATE TABLE transactions (
    id              BIGSERIAL   PRIMARY KEY,
    statement_id    BIGINT      NOT NULL REFERENCES statements (id) ON DELETE CASCADE,
    fingerprint     TEXT        NOT NULL,
    op_at           TIMESTAMPTZ NOT NULL,
    op_date         DATE        NOT NULL,
    processed_on    DATE        NOT NULL,
    card            TEXT        NOT NULL DEFAULT '',
    amount          BIGINT      NOT NULL CHECK (amount > 0),
    direction       TEXT        NOT NULL CHECK (direction IN ('expense', 'income')),
    kind            TEXT        NOT NULL,
    description     TEXT        NOT NULL,
    merchant        TEXT        NOT NULL DEFAULT '',
    mcc             TEXT        NOT NULL DEFAULT '',
    counterparty    TEXT        NOT NULL DEFAULT '',
    category_id     BIGINT      REFERENCES categories (id) ON DELETE SET NULL,
    category_source TEXT        NOT NULL DEFAULT '' CHECK (category_source IN ('', 'rule', 'manual')),
    note            TEXT        NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL,
    CONSTRAINT transactions_fingerprint_key UNIQUE (fingerprint)
);

CREATE INDEX transactions_op_date_idx ON transactions (op_date);
CREATE INDEX transactions_category_id_idx ON transactions (category_id);
CREATE INDEX transactions_statement_id_idx ON transactions (statement_id);
