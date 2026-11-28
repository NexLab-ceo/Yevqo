CREATE TABLE IF NOT EXISTS system_logs (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    level VARCHAR(12) NOT NULL,
    category VARCHAR(48) NOT NULL,
    source VARCHAR(96) NOT NULL,
    message VARCHAR(512) NOT NULL,
    request_id VARCHAR(64) NULL,
    method VARCHAR(12) NULL,
    path VARCHAR(512) NULL,
    status_code SMALLINT UNSIGNED NULL,
    duration_ms BIGINT UNSIGNED NULL,
    metadata JSON NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_system_logs_created (created_at, id),
    INDEX idx_system_logs_level_created (level, created_at),
    INDEX idx_system_logs_category_created (category, created_at),
    INDEX idx_system_logs_request_id (request_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
