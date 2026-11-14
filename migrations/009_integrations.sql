-- External Git provider connections. Tokens are encrypted by Control Plane AES-GCM.
CREATE TABLE IF NOT EXISTS integrations (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT UNSIGNED NOT NULL,
    provider VARCHAR(24) NOT NULL,
    account_id VARCHAR(191) NOT NULL,
    account_name VARCHAR(191) NOT NULL,
    account_email VARCHAR(191) NULL,
    token_nonce VARBINARY(32) NOT NULL,
    token_ciphertext LONGBLOB NOT NULL,
    scopes VARCHAR(512) NOT NULL DEFAULT '',
    connected_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_verified_at TIMESTAMP NULL,
    UNIQUE KEY uq_integrations_user_provider_account (user_id, provider, account_id),
    KEY idx_integrations_user_provider (user_id, provider),
    CONSTRAINT fk_integrations_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
