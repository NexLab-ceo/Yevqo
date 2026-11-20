-- Persist metric alert transitions so repeated samples do not spam notifications.
CREATE TABLE IF NOT EXISTS device_alert_state (
    device_id BIGINT UNSIGNED NOT NULL,
    user_id BIGINT UNSIGNED NOT NULL,
    alert_key VARCHAR(32) NOT NULL,
    active TINYINT(1) NOT NULL DEFAULT 0,
    last_value DECIMAL(10,2) NOT NULL DEFAULT 0,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (device_id, alert_key),
    KEY idx_device_alert_state_user (user_id),
    CONSTRAINT fk_device_alert_state_device FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE,
    CONSTRAINT fk_device_alert_state_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
