-- GitHub push webhook configuration and replay protection.
-- Secrets are AES-GCM encrypted by the Control Plane; delivery IDs are unique per site.
ALTER TABLE sites
    ADD COLUMN github_webhook_secret_enc LONGBLOB NULL,
    ADD COLUMN github_webhook_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN github_webhook_updated_at TIMESTAMP NULL;

CREATE TABLE IF NOT EXISTS github_webhook_deliveries (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    device_id BIGINT UNSIGNED NOT NULL,
    site_id VARCHAR(64) NOT NULL,
    delivery_id VARCHAR(128) NOT NULL,
    received_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    status ENUM('accepted','ignored','failed','completed') NOT NULL DEFAULT 'accepted',
    KEY idx_github_delivery_site (device_id, site_id, received_at),
    UNIQUE KEY uq_github_delivery (device_id, site_id, delivery_id),
    CONSTRAINT fk_github_delivery_site FOREIGN KEY (device_id, site_id) REFERENCES sites(device_id, site_id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
