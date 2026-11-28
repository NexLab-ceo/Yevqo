-- Cloudflare OAuth state and refresh-token storage.
ALTER TABLE cloudflare_accounts
    ADD COLUMN auth_method VARCHAR(16) NOT NULL DEFAULT 'api_token',
    ADD COLUMN oauth_refresh_nonce VARBINARY(32) NULL,
    ADD COLUMN oauth_refresh_ciphertext LONGBLOB NULL,
    ADD COLUMN oauth_expires_at DATETIME NULL;

CREATE TABLE IF NOT EXISTS cloudflare_oauth_states (
    state_hash CHAR(64) NOT NULL PRIMARY KEY,
    user_id BIGINT UNSIGNED NOT NULL,
    return_to VARCHAR(1024) NOT NULL,
    expires_at DATETIME NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    KEY idx_cloudflare_oauth_state_user (user_id),
    KEY idx_cloudflare_oauth_state_expiry (expires_at),
    CONSTRAINT fk_cloudflare_oauth_state_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
