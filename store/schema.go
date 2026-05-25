package store

const schema = `
CREATE TABLE IF NOT EXISTS readings (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    account     TEXT    NOT NULL,
    value       INTEGER NOT NULL,
    trend       TEXT    NOT NULL,
    recorded_at DATETIME NOT NULL,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_readings_account_time ON readings (account, recorded_at DESC);

CREATE TABLE IF NOT EXISTS alarm_state (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    account            TEXT    NOT NULL,
    alarm_name         TEXT    NOT NULL,
    recipient          TEXT    NOT NULL,
    last_fired_at      DATETIME,
    snoozed_until      DATETIME,
    receipt_id         TEXT,
    receipt_expires_at DATETIME,
    rearmed            INTEGER NOT NULL DEFAULT 0,
    UNIQUE (account, alarm_name, recipient)
);
CREATE INDEX IF NOT EXISTS idx_alarm_state_receipt_id ON alarm_state (receipt_id);

CREATE TABLE IF NOT EXISTS alarm_history (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    account     TEXT    NOT NULL,
    alarm_name  TEXT    NOT NULL,
    recipient   TEXT    NOT NULL,
    fired_at    DATETIME NOT NULL,
    bg_value    INTEGER NOT NULL,
    UNIQUE (account, alarm_name, recipient, fired_at)
);
CREATE INDEX IF NOT EXISTS idx_alarm_history_account_time ON alarm_history (account, fired_at DESC);
`
