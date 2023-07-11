CREATE TABLE IF NOT EXISTS bsv20 (
    txid BYTEA,
    vout INT,
    height INT,
    idx BIGINT,
    tick TEXT,
    max NUMERIC NOT NULL,
    lim NUMERIC NOT NULL,
    dec INT DEFAULT 18,
    supply NUMERIC DEFAULT 0,
    valid BOOL,
    available NUMERIC GENERATED ALWAYS AS (max - supply) STORED,
    pct_minted NUMERIC GENERATED ALWAYS AS (CASE WHEN max = 0 THEN 0 ELSE ROUND(100.0 * supply / max, 1) END) STORED,
    reason TEXT,
    PRIMARY KEY(txid, vout)
);
CREATE INDEX IF NOT EXISTS idx_bsv20_tick ON bsv20(tick);
CREATE INDEX IF NOT EXISTS idx_bsv20_available ON bsv20(available);
CREATE INDEX IF NOT EXISTS idx_bsv20_pct_minted ON bsv20(pct_minted);
CREATE INDEX IF NOT EXISTS idx_bsv20_max ON bsv20(max);
CREATE INDEX IF NOT EXISTS idx_bsv20_height_idx ON bsv20(height, idx);

CREATE TABLE IF NOT EXISTS bsv20_txos (
    txid BYTEA,
    vout INT,
    height INT,
    idx BIGINT,
    tick TEXT,
    op TEXT,
    orig_amt NUMERIC NOT NULL,
    amt NUMERIC NOT NULL,
    pkhash BYTEA,
    spend BYTEA DEFAULT ''::bytea,
    valid BOOL,
    implied BOOL DEFAULT FALSE,
    listing BOOLEAN DEFAULT FALSE,
    reason TEXT,
    PRIMARY KEY(txid, vout),
    FOREIGN KEY(txid, vout, spend) REFERENCES txos(txid, vout, spend) ON UPDATE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_bsv20_txos_pkhash ON bsv20_txos(pkhash, valid, spend);
CREATE INDEX IF NOT EXISTS idx_bsv20_txos_spend ON bsv20_txos(spend);
CREATE INDEX IF NOT EXISTS idx_bsv20_txos_tick_valid_op_height ON bsv20_txos(tick, valid, op, height);
CREATE INDEX IF NOT EXISTS idx_bsv20_txos_pkhash_spend_tick_height_idx ON bsv20_txos(pkhash, spend, tick, height, idx);
CREATE INDEX IF NOT EXISTS idx_bsv20_to_validate ON bsv20_txos(height)
    WHERE valid IS NULL;