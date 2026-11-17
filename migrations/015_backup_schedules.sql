-- Persisted backup schedules. Existing backups are untouched.
CREATE TABLE IF NOT EXISTS backup_schedules (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT UNSIGNED NOT NULL,
    device_id BIGINT UNSIGNED NOT NULL,
    resource_type ENUM('site','database') NOT NULL,
    site_id VARCHAR(64) NULL,
    database_id BIGINT UNSIGNED NULL,
    interval_hours INT UNSIGNED NOT NULL,
    retention_count INT UNSIGNED NOT NULL DEFAULT 10,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    next_run_at TIMESTAMP NOT NULL,
    last_run_at TIMESTAMP NULL,
    last_status VARCHAR(24) NOT NULL DEFAULT 'pending',
    last_error VARCHAR(500) NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    KEY idx_backup_schedules_due (enabled, next_run_at),
    KEY idx_backup_schedules_user (user_id, created_at),
    CONSTRAINT fk_backup_schedules_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_backup_schedules_device FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE,
    CONSTRAINT fk_backup_schedules_database FOREIGN KEY (database_id) REFERENCES db_instances(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
