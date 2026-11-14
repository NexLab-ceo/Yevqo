CREATE TABLE IF NOT EXISTS device_metrics (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    device_id BIGINT UNSIGNED NOT NULL,
    cpu_percent DECIMAL(6,2) NOT NULL DEFAULT 0,
    load_1m DECIMAL(8,3) NOT NULL DEFAULT 0,
    memory_total_bytes BIGINT UNSIGNED NOT NULL DEFAULT 0,
    memory_available_bytes BIGINT UNSIGNED NOT NULL DEFAULT 0,
    storage_total_bytes BIGINT UNSIGNED NOT NULL DEFAULT 0,
    storage_available_bytes BIGINT UNSIGNED NOT NULL DEFAULT 0,
    temperature_celsius DECIMAL(5,2) NULL,
    captured_at TIMESTAMP NOT NULL,
    KEY idx_device_metrics_latest (device_id, captured_at),
    CONSTRAINT fk_device_metrics_device FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
