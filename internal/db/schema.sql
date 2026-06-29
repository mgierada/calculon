-- Timestamps are stored as RFC3339 text so they sort lexicographically.
-- The natural key (provider, account_id, external_id) is what makes imports
-- idempotent: re-importing an overlapping statement conflicts and is skipped.

CREATE TABLE IF NOT EXISTS positions (
    provider       TEXT NOT NULL,
    account_id     TEXT NOT NULL,
    external_id    TEXT NOT NULL,
    symbol         TEXT NOT NULL,
    side           TEXT NOT NULL,
    volume         REAL NOT NULL,
    open_time      TEXT NOT NULL,
    open_price     REAL NOT NULL,
    close_time     TEXT,
    close_price    REAL NOT NULL DEFAULT 0,
    purchase_value REAL NOT NULL DEFAULT 0,
    sale_value     REAL NOT NULL DEFAULT 0,
    commission     REAL NOT NULL DEFAULT 0,
    swap           REAL NOT NULL DEFAULT 0,
    rollover       REAL NOT NULL DEFAULT 0,
    gross_pl       REAL NOT NULL DEFAULT 0,
    comment        TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (provider, account_id, external_id)
);

CREATE INDEX IF NOT EXISTS idx_positions_symbol ON positions (symbol, close_time);

CREATE TABLE IF NOT EXISTS cash_ops (
    provider    TEXT NOT NULL,
    account_id  TEXT NOT NULL,
    external_id TEXT NOT NULL,
    kind        TEXT NOT NULL,
    raw_type    TEXT NOT NULL,
    op_time     TEXT NOT NULL,
    comment     TEXT NOT NULL DEFAULT '',
    symbol      TEXT NOT NULL DEFAULT '',
    amount      REAL NOT NULL DEFAULT 0,
    -- volume and price are set only for stock purchases and sales, where they
    -- are destructured out of the provider's comment. volume is always positive.
    volume      REAL NOT NULL DEFAULT 0,
    price       REAL NOT NULL DEFAULT 0,
    PRIMARY KEY (provider, account_id, external_id)
);

CREATE INDEX IF NOT EXISTS idx_cash_ops_symbol ON cash_ops (kind, symbol);
