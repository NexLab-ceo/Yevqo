-- Per-project-environment variables and secrets. Values are AES-GCM encrypted by
-- the Control Plane credential key and are never returned by the API.
CREATE TABLE IF NOT EXISTS project_environment_vars (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    project_id BIGINT UNSIGNED NOT NULL,
    environment_id BIGINT UNSIGNED NOT NULL,
    name VARCHAR(64) NOT NULL,
    nonce VARBINARY(32) NOT NULL,
    ciphertext BLOB NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uq_project_environment_var (environment_id, name),
    KEY idx_project_environment_vars_project (project_id, environment_id),
    CONSTRAINT fk_project_environment_vars_project FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
    CONSTRAINT fk_project_environment_vars_environment FOREIGN KEY (environment_id) REFERENCES project_environments(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
