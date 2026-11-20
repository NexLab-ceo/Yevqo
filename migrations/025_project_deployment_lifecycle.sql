-- Project GitHub delivery targets and durable deployment lifecycle.
-- Additive only; no existing site, release, or database data is rewritten.

CREATE TABLE IF NOT EXISTS project_repository_sites (
    project_id BIGINT UNSIGNED NOT NULL,
    device_id BIGINT UNSIGNED NOT NULL,
    site_id VARCHAR(64) NOT NULL,
    auto_deploy_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    deploy_branch VARCHAR(255) NOT NULL DEFAULT '',
    previous_git_url VARCHAR(512) NULL,
    previous_git_branch VARCHAR(255) NOT NULL DEFAULT 'main',
    previous_webhook_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    previous_webhook_secret_enc LONGBLOB NULL,
    github_webhook_id BIGINT UNSIGNED NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (project_id, device_id, site_id),
    UNIQUE KEY uq_project_repository_site_target (device_id, site_id),
    CONSTRAINT fk_project_repository_sites_repository FOREIGN KEY (project_id) REFERENCES project_repositories(project_id) ON DELETE CASCADE,
    CONSTRAINT fk_project_repository_sites_project_site FOREIGN KEY (project_id, device_id, site_id) REFERENCES project_sites(project_id, device_id, site_id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS project_deployment_runs (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    project_id BIGINT UNSIGNED NULL,
    user_id BIGINT UNSIGNED NOT NULL,
    device_id BIGINT UNSIGNED NOT NULL,
    site_id VARCHAR(64) NOT NULL,
    source VARCHAR(24) NOT NULL DEFAULT 'manual',
    branch VARCHAR(255) NOT NULL DEFAULT '',
    commit_sha VARCHAR(64) NOT NULL DEFAULT '',
    delivery_id VARCHAR(128) NULL,
    agent_job_id VARCHAR(64) NOT NULL DEFAULT '',
    status VARCHAR(24) NOT NULL DEFAULT 'queued',
    progress TINYINT UNSIGNED NOT NULL DEFAULT 0,
    message VARCHAR(1000) NOT NULL DEFAULT '',
    output MEDIUMTEXT NULL,
    started_at TIMESTAMP NULL,
    finished_at TIMESTAMP NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    KEY idx_project_deployments_project_created (project_id, created_at),
    KEY idx_project_deployments_site_created (device_id, site_id, created_at),
    KEY idx_project_deployments_user_created (user_id, created_at),
    KEY idx_project_deployments_agent_job (device_id, site_id, agent_job_id),
    UNIQUE KEY uq_project_deployments_delivery (device_id, site_id, delivery_id),
    CONSTRAINT fk_project_deployments_project FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE SET NULL,
    CONSTRAINT fk_project_deployments_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_project_deployments_device FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
