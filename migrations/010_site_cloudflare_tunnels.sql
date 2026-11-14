-- Persist Cloudflare Tunnel runtime state and encrypted token (additive only).
ALTER TABLE sites
  ADD COLUMN tunnel_status VARCHAR(16) NOT NULL DEFAULT 'inactive',
  ADD COLUMN tunnel_error VARCHAR(512) NULL,
  ADD COLUMN tunnel_token_enc BLOB NULL,
  ADD COLUMN tunnel_pid INT UNSIGNED NULL,
  ADD COLUMN tunnel_updated_at TIMESTAMP NULL;
