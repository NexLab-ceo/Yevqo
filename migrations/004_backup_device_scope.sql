ALTER TABLE backups
    ADD COLUMN device_id BIGINT UNSIGNED NULL,
    ADD KEY idx_backups_device (device_id),
    ADD CONSTRAINT fk_backups_device FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE SET NULL;
