ALTER TABLE transactions ADD COLUMN excluded BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE budgets (
    id          BIGSERIAL   PRIMARY KEY,
    category_id BIGINT      REFERENCES categories (id) ON DELETE CASCADE,
    amount      BIGINT      NOT NULL CHECK (amount > 0),
    created_at  TIMESTAMPTZ NOT NULL,
    updated_at  TIMESTAMPTZ NOT NULL,
    -- A NULL category is the overall monthly limit; NULLS NOT DISTINCT keeps
    -- it single so the upsert has one row to hit.
    CONSTRAINT budgets_category_key UNIQUE NULLS NOT DISTINCT (category_id)
);
