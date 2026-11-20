-- Store Cloudflare account ID and managed per-site remotely-managed tunnel routes.
ALTER TABLE cloudflare_accounts
    ADD COLUMN account_id VARCHAR(32) NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS site_cloudflare_routes (
    device_id BIGINT UNSIGNED NOT NULL,
    site_id VARCHAR(64) NOT NULL,
    user_id BIGINT UNSIGNED NOT NULL,
    tunnel_id CHAR(36) NOT NULL,
    hostname VARCHAR(253) NOT NULL,
    origin_port INT UNSIGNED NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (device_id, site_id),
    UNIQUE KEY uq_site_cloudflare_routes_user_hostname (user_id, hostname),
    KEY idx_site_cloudflare_routes_tunnel (user_id, tunnel_id),
    CONSTRAINT fk_site_cloudflare_routes_device FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE,
    CONSTRAINT fk_site_cloudflare_routes_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
