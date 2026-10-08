-- Timestamps are stored as RFC3339 text in UTC so they sort lexicographically.
--
-- History tables (positions, cash_ops, quotes) are keyed by a natural key and
-- carry a content_hash of every other column. Importing a record whose key is
-- known compares hashes: equal is skipped, different is updated in place.
-- open_lots is a snapshot instead, replaced whenever a newer statement arrives.

CREATE TABLE IF NOT EXISTS users (
    id         INTEGER PRIMARY KEY,
    name       TEXT NOT NULL UNIQUE,
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS user_keys (
    -- SHA256 fingerprint in OpenSSH's "SHA256:..." form.
    fingerprint TEXT PRIMARY KEY,
    user_id     INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    public_key  TEXT NOT NULL,
    comment     TEXT NOT NULL DEFAULT '',
    added_at    TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS accounts (
    provider       TEXT NOT NULL,
    account_id     TEXT NOT NULL,
    user_id        INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    currency       TEXT NOT NULL,
    -- A human label such as "IKE"; set from the statement on first import and
    -- changed with `calculon account rename`.
    name           TEXT NOT NULL DEFAULT '',
    -- When the stored open_lots snapshot was taken; NULL before the first one.
    snapshot_as_of TEXT,
    PRIMARY KEY (provider, account_id)
);

CREATE INDEX IF NOT EXISTS idx_accounts_user ON accounts (user_id);

CREATE TABLE IF NOT EXISTS positions (
    provider       TEXT NOT NULL,
    account_id     TEXT NOT NULL,
    row_key        TEXT NOT NULL,
    position_id    TEXT NOT NULL,
    seq            INTEGER NOT NULL,
    symbol         TEXT NOT NULL,
    name           TEXT NOT NULL,
    category       TEXT NOT NULL,
    product        TEXT NOT NULL,
    side           TEXT NOT NULL,
    volume         REAL NOT NULL,
    open_time      TEXT NOT NULL,
    open_price     REAL NOT NULL,
    close_time     TEXT NOT NULL,
    close_price    REAL NOT NULL,
    purchase_value REAL NOT NULL,
    sale_value     REAL NOT NULL,
    commission     REAL NOT NULL,
    swap           REAL NOT NULL,
    rollover       REAL NOT NULL,
    gross_pl       REAL NOT NULL,
    net_pl         REAL NOT NULL,
    close_origin   TEXT NOT NULL,
    comment        TEXT NOT NULL,
    content_hash   TEXT NOT NULL,
    PRIMARY KEY (provider, account_id, row_key),
    FOREIGN KEY (provider, account_id) REFERENCES accounts (provider, account_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_positions_symbol ON positions (symbol, close_time);

CREATE TABLE IF NOT EXISTS cash_ops (
    provider     TEXT NOT NULL,
    account_id   TEXT NOT NULL,
    external_id  TEXT NOT NULL,
    kind         TEXT NOT NULL,
    raw_type     TEXT NOT NULL,
    op_time      TEXT NOT NULL,
    symbol       TEXT NOT NULL,
    name         TEXT NOT NULL,
    category     TEXT NOT NULL,
    product      TEXT NOT NULL,
    position_id  TEXT NOT NULL,
    comment      TEXT NOT NULL,
    amount       REAL NOT NULL,
    -- volume and price are set only for stock purchases and sales, where they
    -- are destructured out of the provider's comment. volume is always positive.
    volume       REAL NOT NULL,
    price        REAL NOT NULL,
    content_hash TEXT NOT NULL,
    PRIMARY KEY (provider, account_id, external_id),
    FOREIGN KEY (provider, account_id) REFERENCES accounts (provider, account_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_cash_ops_time ON cash_ops (op_time);

CREATE TABLE IF NOT EXISTS open_lots (
    provider      TEXT NOT NULL,
    account_id    TEXT NOT NULL,
    row_key       TEXT NOT NULL,
    position_id   TEXT NOT NULL,
    seq           INTEGER NOT NULL,
    symbol        TEXT NOT NULL,
    name          TEXT NOT NULL,
    category      TEXT NOT NULL,
    product       TEXT NOT NULL,
    side          TEXT NOT NULL,
    volume        REAL NOT NULL,
    open_time     TEXT NOT NULL,
    open_price    REAL NOT NULL,
    current_price REAL NOT NULL,
    value         REAL NOT NULL,
    gross_pl      REAL NOT NULL,
    net_pl        REAL NOT NULL,
    commission    REAL NOT NULL,
    swap          REAL NOT NULL,
    PRIMARY KEY (provider, account_id, row_key),
    FOREIGN KEY (provider, account_id) REFERENCES accounts (provider, account_id) ON DELETE CASCADE
);

-- Observed prices. Statements add one per held symbol at their snapshot time;
-- a market data source can add more under its own source name.
CREATE TABLE IF NOT EXISTS quotes (
    symbol       TEXT NOT NULL,
    as_of        TEXT NOT NULL,
    price        REAL NOT NULL,
    source       TEXT NOT NULL,
    content_hash TEXT NOT NULL,
    -- The previous session's close as the source reported it, which day
    -- change is measured from; NULL when the source gives none, as statements.
    prev_close   REAL,
    PRIMARY KEY (symbol, as_of)
);

-- How a market data provider names each of our symbols, e.g. XTB.PL is
-- XTB.WA at finimpulse. Rows are seeded from suffix rules and never
-- overwritten, so a mapping fixed by hand sticks.
CREATE TABLE IF NOT EXISTS symbol_map (
    symbol          TEXT NOT NULL,
    provider        TEXT NOT NULL,
    provider_symbol TEXT NOT NULL,
    PRIMARY KEY (symbol, provider)
);

-- Every finimpulse market price response, kept whole. symbol is ours,
-- provider_symbol the one finimpulse answered for; fetched_at is our clock.
-- Nullable columns are null when the API sent null.
CREATE TABLE IF NOT EXISTS intraday_price (
    id                                  INTEGER PRIMARY KEY,
    symbol                              TEXT NOT NULL,
    fetched_at                          TEXT NOT NULL,
    task_id                             TEXT NOT NULL,
    status_code                         INTEGER NOT NULL,
    status_message                      TEXT NOT NULL,
    live                                INTEGER NOT NULL,
    cost                                REAL NOT NULL,
    provider_symbol                     TEXT NOT NULL,
    name                                TEXT NOT NULL,
    quote_type                          TEXT NOT NULL,
    currency                            TEXT NOT NULL,
    regular_market_volume               INTEGER,
    market_cap                          INTEGER,
    usd_rate                            REAL,
    market_state                        TEXT NOT NULL,
    regular_market_open                 REAL,
    regular_market_previous_close       REAL,
    current_price                       REAL,
    current_price_usd                   REAL,
    current_price_change                REAL,
    current_price_change_percent        REAL,
    current_price_update_time           TEXT,
    regular_market_price                REAL,
    regular_market_price_change         REAL,
    regular_market_price_change_percent REAL,
    regular_market_time                 TEXT,
    pre_market_price                    REAL,
    pre_market_price_change             REAL,
    pre_market_price_change_percent     REAL,
    pre_market_time                     TEXT,
    post_market_price                   REAL,
    post_market_price_change            REAL,
    post_market_price_change_percent    REAL,
    post_market_time                    TEXT
);

CREATE INDEX IF NOT EXISTS idx_intraday_price_symbol ON intraday_price (symbol, fetched_at);

-- The last price of each symbol's session, one row per exchange-local trading
-- day. Every fetch overwrites its session's row, so the row for today is
-- provisional and earlier days hold the close.
CREATE TABLE IF NOT EXISTS eod_price (
    symbol       TEXT NOT NULL,
    session_date TEXT NOT NULL,
    close        REAL NOT NULL,
    currency     TEXT NOT NULL,
    -- When the exchange printed the price, and when we fetched it.
    as_of        TEXT NOT NULL,
    fetched_at   TEXT NOT NULL,
    PRIMARY KEY (symbol, session_date)
);

-- Articles from the finimpulse news endpoint, every field kept. An article
-- is stored once however many held symbols it was found for.
CREATE TABLE IF NOT EXISTS news_item (
    id                    TEXT PRIMARY KEY,
    type                  TEXT NOT NULL,
    title                 TEXT NOT NULL,
    description           TEXT NOT NULL,
    pub_date              TEXT NOT NULL,
    display_time          TEXT,
    canonical_url         TEXT NOT NULL,
    content_type          TEXT NOT NULL,
    -- JSON array of tickers in the provider's notation.
    related_tickers       TEXT NOT NULL,
    provider_display_name TEXT NOT NULL,
    provider_url          TEXT NOT NULL,
    is_hosted             INTEGER NOT NULL,
    is_premium_news       INTEGER NOT NULL,
    fetched_at            TEXT NOT NULL
);

-- Which of our symbols each article was fetched for.
CREATE TABLE IF NOT EXISTS news_symbol (
    symbol  TEXT NOT NULL,
    news_id TEXT NOT NULL REFERENCES news_item (id) ON DELETE CASCADE,
    PRIMARY KEY (symbol, news_id)
);

-- Analysts' price targets, one row per earnings fetch so their history is
-- kept; symbol is ours.
CREATE TABLE IF NOT EXISTS earnings_target (
    id                   INTEGER PRIMARY KEY,
    symbol               TEXT NOT NULL,
    fetched_at           TEXT NOT NULL,
    target_price         REAL,
    target_average_price REAL,
    target_low_price     REAL,
    target_high_price    REAL,
    total_count          INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_earnings_target_symbol ON earnings_target (symbol, fetched_at);

-- Earnings items, one row per period, overwritten when a fetch reports a
-- period again so revised figures win. date is the period's first day.
CREATE TABLE IF NOT EXISTS earnings_growth (
    symbol           TEXT NOT NULL,
    date             TEXT NOT NULL,
    date_type        TEXT NOT NULL,
    growth           REAL,
    growth_benchmark REAL,
    symbol_benchmark TEXT NOT NULL,
    fetched_at       TEXT NOT NULL,
    PRIMARY KEY (symbol, date, date_type)
);

CREATE TABLE IF NOT EXISTS earnings_eps (
    symbol       TEXT NOT NULL,
    date         TEXT NOT NULL,
    date_type    TEXT NOT NULL,
    methodology  TEXT NOT NULL,
    actual       REAL,
    estimate     REAL,
    surprise     REAL,
    surprise_pct REAL,
    fetched_at   TEXT NOT NULL,
    PRIMARY KEY (symbol, date, date_type, methodology)
);

CREATE TABLE IF NOT EXISTS earnings_revenue (
    symbol      TEXT NOT NULL,
    date        TEXT NOT NULL,
    date_type   TEXT NOT NULL,
    methodology TEXT NOT NULL,
    revenue     REAL,
    earnings    REAL,
    fetched_at  TEXT NOT NULL,
    PRIMARY KEY (symbol, date, date_type, methodology)
);

-- Analyst recommendations, one row per month, overwritten when a fetch
-- reports a month again so revised counts win. date is the month's first day.
CREATE TABLE IF NOT EXISTS recommendation (
    symbol      TEXT NOT NULL,
    date        TEXT NOT NULL,
    strong_buy  INTEGER NOT NULL,
    buy         INTEGER NOT NULL,
    hold        INTEGER NOT NULL,
    sell        INTEGER NOT NULL,
    strong_sell INTEGER NOT NULL,
    fetched_at  TEXT NOT NULL,
    PRIMARY KEY (symbol, date)
);
