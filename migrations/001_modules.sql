-- VELTRIX Dock module foundation.
-- Additive-only schema. Existing users, devices, sites and their data are preserved.

CREATE TABLE IF NOT EXISTS projects (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT UNSIGNED NOT NULL,
    name VARCHAR(80) NOT NULL,
    slug VARCHAR(64) NOT NULL,
    description VARCHAR(500) NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uq_projects_user_slug (user_id, slug),
    CONSTRAINT fk_projects_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS project_devices (
    project_id BIGINT UNSIGNED NOT NULL,
    device_id BIGINT UNSIGNED NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (project_id, device_id),
    CONSTRAINT fk_project_devices_project FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    CONSTRAINT fk_project_devices_device FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS project_sites (
    project_id BIGINT UNSIGNED NOT NULL,
    device_id BIGINT UNSIGNED NOT NULL,
    site_id VARCHAR(64) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (project_id, device_id, site_id),
    CONSTRAINT fk_project_sites_project FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    CONSTRAINT fk_project_sites_device FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS database_servers (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT UNSIGNED NOT NULL,
    device_id BIGINT UNSIGNED NOT NULL,
    name VARCHAR(64) NOT NULL,
    engine ENUM('postgresql','mariadb','mysql') NOT NULL,
    version VARCHAR(32) NOT NULL DEFAULT '',
    port INT UNSIGNED NOT NULL,
    data_directory VARCHAR(255) NOT NULL,
    network_access ENUM('private','localhost') NOT NULL DEFAULT 'private',
    status ENUM('planned','installing','ready','stopped','error','removing') NOT NULL DEFAULT 'planned',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uq_database_server_name (user_id, device_id, name),
    UNIQUE KEY uq_database_server_port (device_id, port),
    CONSTRAINT fk_database_servers_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_database_servers_device FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS db_instances (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT UNSIGNED NOT NULL,
    server_id BIGINT UNSIGNED NOT NULL,
    name VARCHAR(63) NOT NULL,
    status ENUM('planned','ready','error') NOT NULL DEFAULT 'planned',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uq_database_name (server_id, name),
    CONSTRAINT fk_databases_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_databases_server FOREIGN KEY (server_id) REFERENCES database_servers(id) ON DELETE RESTRICT
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS user_settings (
    user_id BIGINT UNSIGNED NOT NULL,
    setting_key VARCHAR(64) NOT NULL,
    setting_value JSON NOT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, setting_key),
    CONSTRAINT fk_user_settings_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS notifications (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT UNSIGNED NOT NULL,
    type VARCHAR(32) NOT NULL DEFAULT 'info',
    title VARCHAR(160) NOT NULL,
    body VARCHAR(1000) NOT NULL DEFAULT '',
    read_at TIMESTAMP NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    KEY idx_notifications_user_created (user_id, created_at),
    CONSTRAINT fk_notifications_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS backups (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT UNSIGNED NOT NULL,
    resource_type VARCHAR(32) NOT NULL,
    resource_id VARCHAR(128) NOT NULL,
    file_name VARCHAR(255) NOT NULL,
    size_bytes BIGINT UNSIGNED NOT NULL DEFAULT 0,
    status ENUM('queued','running','completed','failed','verified') NOT NULL DEFAULT 'queued',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    KEY idx_backups_user_created (user_id, created_at),
    CONSTRAINT fk_backups_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
