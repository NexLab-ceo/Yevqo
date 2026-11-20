-- Project-level GitHub App repository binding.
-- Additive-only: existing project and site Git settings remain unchanged.

CREATE TABLE IF NOT EXISTS project_repositories (
    project_id BIGINT UNSIGNED NOT NULL PRIMARY KEY,
    integration_id BIGINT UNSIGNED NOT NULL,
    repository_id BIGINT UNSIGNED NOT NULL,
    full_name VARCHAR(255) NOT NULL,
    clone_url VARCHAR(512) NOT NULL,
    web_url VARCHAR(512) NOT NULL,
    default_branch VARCHAR(255) NOT NULL,
    deploy_branch VARCHAR(255) NOT NULL,
    is_private BOOLEAN NOT NULL DEFAULT FALSE,
    language VARCHAR(100) NOT NULL DEFAULT '',
    last_commit_sha VARCHAR(64) NOT NULL DEFAULT '',
    auto_deploy_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    KEY idx_project_repositories_integration (integration_id),
    KEY idx_project_repositories_repo (repository_id),
    CONSTRAINT fk_project_repositories_project FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
