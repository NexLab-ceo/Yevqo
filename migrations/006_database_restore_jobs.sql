ALTER TABLE backups
    ADD COLUMN database_server_id BIGINT UNSIGNED NULL,
    ADD KEY idx_backups_database_server (database_server_id);

CREATE TABLE IF NOT EXISTS database_restore_jobs (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT UNSIGNED NOT NULL,
    backup_id BIGINT UNSIGNED NOT NULL,
    target_database_id BIGINT UNSIGNED NOT NULL,
    status ENUM('queued','running','completed','failed') NOT NULL DEFAULT 'queued',
    error_message VARCHAR(500) NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    KEY idx_database_restore_user (user_id, created_at),
    CONSTRAINT fk_database_restore_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_database_restore_backup FOREIGN KEY (backup_id) REFERENCES backups(id) ON DELETE CASCADE,
    CONSTRAINT fk_database_restore_target FOREIGN KEY (target_database_id) REFERENCES databases(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
