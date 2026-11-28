-- Platform-wide settings managed from the administrator panel.
-- OAuth client secrets are encrypted with the Control Plane credential key.
CREATE TABLE IF NOT EXISTS platform_settings (
    id TINYINT UNSIGNED NOT NULL PRIMARY KEY,
    public_url VARCHAR(512) NOT NULL DEFAULT '',
    cloudflare_oauth_client_id VARCHAR(255) NOT NULL DEFAULT '',
    cloudflare_oauth_secret_nonce VARBINARY(32) NULL,
    cloudflare_oauth_secret_ciphertext LONGBLOB NULL,
    github_app_id VARCHAR(32) NOT NULL DEFAULT '',
    github_app_slug VARCHAR(255) NOT NULL DEFAULT '',
    github_app_private_key_nonce VARBINARY(32) NULL,
    github_app_private_key_ciphertext LONGBLOB NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
