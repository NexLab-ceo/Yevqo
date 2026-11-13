-- Persistent login throttling. Additive-only; preserves all existing data.
CREATE TABLE IF NOT EXISTS login_attempts (
    ip VARCHAR(45) NOT NULL PRIMARY KEY,
    window_started DATETIME NOT NULL,
    attempts INT UNSIGNED NOT NULL DEFAULT 0,
    blocked_until DATETIME NULL,
    KEY idx_login_attempts_blocked_until (blocked_until)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
