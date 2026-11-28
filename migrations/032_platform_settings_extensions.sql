-- Extend platform settings without rewriting migration 030 on existing installations.
ALTER TABLE platform_settings
    ADD COLUMN IF NOT EXISTS cloudflare_oauth_token_auth_method VARCHAR(32) NOT NULL DEFAULT 'client_secret_post',
    ADD COLUMN IF NOT EXISTS github_app_id VARCHAR(32) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS github_app_slug VARCHAR(255) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS github_app_private_key_nonce VARBINARY(32) NULL,
    ADD COLUMN IF NOT EXISTS github_app_private_key_ciphertext LONGBLOB NULL;
