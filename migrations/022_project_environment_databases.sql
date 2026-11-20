-- Add environment-scoped database server assignments.
-- Additive-only: existing projects, databases and environments are preserved.

CREATE TABLE IF NOT EXISTS project_environment_database_servers (
    project_id BIGINT UNSIGNED NOT NULL,
    environment_id BIGINT UNSIGNED NOT NULL,
    database_server_id BIGINT UNSIGNED NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (environment_id, database_server_id),
    KEY idx_project_environment_databases_project (project_id),
    CONSTRAINT fk_project_environment_databases_project FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    CONSTRAINT fk_project_environment_databases_environment FOREIGN KEY (environment_id) REFERENCES project_environments(id) ON DELETE CASCADE,
    CONSTRAINT fk_project_environment_databases_server FOREIGN KEY (database_server_id) REFERENCES database_servers(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
