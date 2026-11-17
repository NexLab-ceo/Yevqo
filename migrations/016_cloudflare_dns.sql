-- Per-user Cloudflare API configuration for DNS automation.
-- API tokens are AES-GCM encrypted by Control Plane.
CREATE TABLE IF NOT EXISTS cloudflare_accounts (
    user_id BIGINT UNSIGNED PRIMARY KEY,
    zone_id VARCHAR(64) NOT NULL,
    zone_name VARCHAR(253) NOT NULL,
    token_nonce VARBINARY(32) NOT NULL,
    token_ciphertext LONGBLOB NOT NULL,
    connected_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_verified_at TIMESTAMP NULL,
    CONSTRAINT fk_cloudflare_account_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
