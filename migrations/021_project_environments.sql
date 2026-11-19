-- Project environments and project-scoped delivery resources.
-- Additive-only: preserves existing projects, devices, sites, databases and build history.

CREATE TABLE IF NOT EXISTS project_environments (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    project_id BIGINT UNSIGNED NOT NULL,
    name VARCHAR(64) NOT NULL,
    slug VARCHAR(64) NOT NULL,
    description VARCHAR(255) NOT NULL DEFAULT '',
    is_protected BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uq_project_environment_slug (project_id, slug),
    KEY idx_project_environments_project (project_id),
    CONSTRAINT fk_project_environments_project FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS project_environment_sites (
    project_id BIGINT UNSIGNED NOT NULL,
    environment_id BIGINT UNSIGNED NOT NULL,
    device_id BIGINT UNSIGNED NOT NULL,
    site_id VARCHAR(64) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (environment_id, device_id, site_id),
    KEY idx_project_environment_sites_project (project_id),
    CONSTRAINT fk_project_environment_sites_project FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    CONSTRAINT fk_project_environment_sites_environment FOREIGN KEY (environment_id) REFERENCES project_environments(id) ON DELETE CASCADE,
    CONSTRAINT fk_project_environment_sites_device FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS project_database_servers (
    project_id BIGINT UNSIGNED NOT NULL,
    database_server_id BIGINT UNSIGNED NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (project_id, database_server_id),
    CONSTRAINT fk_project_database_servers_project FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    CONSTRAINT fk_project_database_servers_server FOREIGN KEY (database_server_id) REFERENCES database_servers(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Give every existing project the standard Dokploy-style environments.
INSERT IGNORE INTO project_environments (project_id, name, slug, description, is_protected)
SELECT id, 'Production', 'production', 'Live production workloads', TRUE FROM projects;

INSERT IGNORE INTO project_environments (project_id, name, slug, description, is_protected)
SELECT id, 'Development', 'development', 'Development and integration workloads', FALSE FROM projects;

INSERT IGNORE INTO project_environments (project_id, name, slug, description, is_protected)
SELECT id, 'Staging', 'staging', 'Pre-production validation workloads', FALSE FROM projects;
