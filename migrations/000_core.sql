-- VELTRIX Dock core schema.
-- This is the first Control Plane migration. It creates only missing core tables.
-- Existing rows and tables are preserved. Additive-only; never DROP or TRUNCATE.

CREATE TABLE IF NOT EXISTS users (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    username VARCHAR(64) NOT NULL UNIQUE,
    email VARCHAR(128) NOT NULL UNIQUE,
    password_hash VARCHAR(255) NOT NULL,
    status ENUM('free','commercial','blocked') NOT NULL DEFAULT 'free',
    referral_code VARCHAR(16) NOT NULL UNIQUE,
    referred_by BIGINT UNSIGNED NULL,
    bonus_devices INT UNSIGNED NOT NULL DEFAULT 0,
    max_devices INT UNSIGNED NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_seen TIMESTAMP NULL,
    KEY idx_users_status (status),
    KEY idx_users_referral (referral_code),
    KEY idx_users_referred_by (referred_by),
    CONSTRAINT fk_users_referred_by FOREIGN KEY (referred_by) REFERENCES users(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS devices (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT UNSIGNED NOT NULL,
    device_id CHAR(36) NOT NULL UNIQUE,
    hostname VARCHAR(128) NULL,
    tailscale_ip VARCHAR(15) NULL,
    agent_version VARCHAR(16) NULL,
    status ENUM('online','offline') NOT NULL DEFAULT 'offline',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_seen TIMESTAMP NULL,
    KEY idx_devices_user (user_id),
    KEY idx_devices_status (status),
    KEY idx_devices_last_seen (last_seen),
    CONSTRAINT fk_devices_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS sites (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    device_id BIGINT UNSIGNED NOT NULL,
    site_id VARCHAR(64) NOT NULL,
    path VARCHAR(255) NOT NULL,
    port INT UNSIGNED NOT NULL,
    runtime ENUM('php','go','static') NOT NULL DEFAULT 'php',
    domain VARCHAR(128) NULL,
    status ENUM('running','stopped','error') NOT NULL DEFAULT 'stopped',
    pid INT UNSIGNED NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uq_sites_device_site (device_id, site_id),
    KEY idx_sites_device (device_id),
    KEY idx_sites_status (status),
    CONSTRAINT fk_sites_device FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS sessions (
    id CHAR(64) PRIMARY KEY,
    user_id BIGINT UNSIGNED NOT NULL,
    ip VARCHAR(45) NULL,
    user_agent VARCHAR(255) NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL,
    KEY idx_sessions_user (user_id),
    KEY idx_sessions_expires (expires_at),
    CONSTRAINT fk_sessions_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS logs (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT UNSIGNED NULL,
    device_id BIGINT UNSIGNED NULL,
    action VARCHAR(64) NOT NULL,
    details JSON NULL,
    ip VARCHAR(45) NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    KEY idx_logs_user (user_id),
    KEY idx_logs_action (action),
    KEY idx_logs_created (created_at),
    CONSTRAINT fk_logs_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL,
    CONSTRAINT fk_logs_device FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
