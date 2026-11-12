-- Database credentials are encrypted by the Control Plane; plaintext passwords are never stored.
CREATE TABLE IF NOT EXISTS database_credentials (
    database_id BIGINT UNSIGNED NOT NULL PRIMARY KEY,
    username VARCHAR(63) NOT NULL,
    ciphertext LONGBLOB NOT NULL,
    nonce VARBINARY(32) NOT NULL,
    key_version SMALLINT UNSIGNED NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    CONSTRAINT fk_database_credentials_database FOREIGN KEY (database_id) REFERENCES databases(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- A site-to-database link is independent from both resources: deleting a site must not delete its database.
CREATE TABLE IF NOT EXISTS site_database_connections (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT UNSIGNED NOT NULL,
    device_id BIGINT UNSIGNED NOT NULL,
    site_id VARCHAR(64) NOT NULL,
    database_id BIGINT UNSIGNED NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uq_site_database (device_id, site_id, database_id),
    KEY idx_site_database_user (user_id, device_id, site_id),
    CONSTRAINT fk_site_database_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_site_database_database FOREIGN KEY (database_id) REFERENCES databases(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
