-- Per-site environment variables. Values are encrypted by Control Plane AES-GCM.
-- Additive-only: no existing site or user data is modified.
CREATE TABLE IF NOT EXISTS site_environment_vars (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT UNSIGNED NOT NULL,
    device_id BIGINT UNSIGNED NOT NULL,
    site_id VARCHAR(64) NOT NULL,
    name VARCHAR(64) NOT NULL,
    nonce VARBINARY(32) NOT NULL,
    ciphertext LONGBLOB NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uq_site_environment_name (user_id, device_id, site_id, name),
    KEY idx_site_environment_scope (device_id, site_id),
    CONSTRAINT fk_site_environment_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_site_environment_site FOREIGN KEY (device_id, site_id) REFERENCES sites(device_id, site_id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
