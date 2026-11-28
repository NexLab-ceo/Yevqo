ALTER TABLE system_logs
    ADD COLUMN user_id BIGINT UNSIGNED NULL AFTER request_id,
    ADD COLUMN device_id BIGINT UNSIGNED NULL AFTER user_id,
    ADD INDEX idx_system_logs_user_created (user_id, created_at, id),
    ADD INDEX idx_system_logs_device_created (device_id, created_at, id);
