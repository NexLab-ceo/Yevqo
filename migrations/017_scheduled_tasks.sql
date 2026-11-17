-- Persistent per-site scheduled commands. Commands are explicit user-configured tasks.
CREATE TABLE IF NOT EXISTS scheduled_tasks (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT UNSIGNED NOT NULL,
    device_id BIGINT UNSIGNED NOT NULL,
    site_id VARCHAR(64) NOT NULL,
    name VARCHAR(100) NOT NULL,
    command TEXT NOT NULL,
    interval_minutes INT UNSIGNED NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    next_run_at TIMESTAMP NOT NULL,
    last_run_at TIMESTAMP NULL,
    last_status VARCHAR(24) NOT NULL DEFAULT 'pending',
    last_output MEDIUMTEXT NULL,
    last_error VARCHAR(500) NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    KEY idx_scheduled_tasks_due (enabled,next_run_at),
    KEY idx_scheduled_tasks_site (device_id,site_id),
    CONSTRAINT fk_scheduled_tasks_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_scheduled_tasks_device FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE,
    CONSTRAINT fk_scheduled_tasks_site FOREIGN KEY (device_id,site_id) REFERENCES sites(device_id,site_id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
