-- P2 platform foundations.
-- Additive-only: no existing tables or rows are dropped or rewritten.

CREATE TABLE IF NOT EXISTS preview_deployments (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT UNSIGNED NOT NULL,
    device_id BIGINT UNSIGNED NOT NULL,
    site_id VARCHAR(64) NOT NULL,
    preview_key VARCHAR(64) NOT NULL,
    source_branch VARCHAR(255) NOT NULL DEFAULT '',
    source_commit VARCHAR(64) NOT NULL DEFAULT '',
    runtime VARCHAR(32) NOT NULL DEFAULT '',
    path VARCHAR(512) NOT NULL,
    port INT UNSIGNED NOT NULL,
    pid INT UNSIGNED NULL,
    status ENUM('queued','building','starting','ready','failed','stopped','expired') NOT NULL DEFAULT 'queued',
    error_message VARCHAR(1000) NOT NULL DEFAULT '',
    expires_at TIMESTAMP NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uq_preview_key (device_id, site_id, preview_key),
    KEY idx_previews_user_status (user_id, status, created_at),
    KEY idx_previews_expiry (status, expires_at),
    CONSTRAINT fk_previews_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_previews_device FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS build_executors (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT UNSIGNED NOT NULL,
    device_id BIGINT UNSIGNED NULL,
    name VARCHAR(100) NOT NULL,
    platform VARCHAR(32) NOT NULL DEFAULT 'linux',
    architecture VARCHAR(32) NOT NULL DEFAULT '',
    status ENUM('offline','online','draining','disabled') NOT NULL DEFAULT 'offline',
    capabilities JSON NULL,
    max_concurrent_jobs INT UNSIGNED NOT NULL DEFAULT 1,
    last_seen TIMESTAMP NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    KEY idx_build_executors_user_status (user_id, status),
    CONSTRAINT fk_build_executors_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_build_executors_device FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS build_jobs (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT UNSIGNED NOT NULL,
    executor_id BIGINT UNSIGNED NULL,
    device_id BIGINT UNSIGNED NULL,
    site_id VARCHAR(64) NOT NULL DEFAULT '',
    provider VARCHAR(32) NOT NULL,
    source_ref VARCHAR(255) NOT NULL DEFAULT '',
    status ENUM('queued','assigned','preparing','building','succeeded','failed','cancelled') NOT NULL DEFAULT 'queued',
    progress TINYINT UNSIGNED NOT NULL DEFAULT 0,
    output MEDIUMTEXT NULL,
    error_message VARCHAR(1000) NOT NULL DEFAULT '',
    started_at TIMESTAMP NULL,
    finished_at TIMESTAMP NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    KEY idx_build_jobs_user_created (user_id, created_at),
    KEY idx_build_jobs_executor_status (executor_id, status),
    CONSTRAINT fk_build_jobs_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_build_jobs_executor FOREIGN KEY (executor_id) REFERENCES build_executors(id) ON DELETE SET NULL,
    CONSTRAINT fk_build_jobs_device FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS app_templates (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    owner_user_id BIGINT UNSIGNED NULL,
    template_key VARCHAR(100) NOT NULL,
    name VARCHAR(120) NOT NULL,
    description VARCHAR(500) NOT NULL DEFAULT '',
    runtime VARCHAR(32) NOT NULL,
    provider VARCHAR(32) NOT NULL DEFAULT 'auto',
    repository_url VARCHAR(512) NOT NULL DEFAULT '',
    starter_archive VARCHAR(512) NOT NULL DEFAULT '',
    variables_schema JSON NULL,
    default_port INT UNSIGNED NOT NULL DEFAULT 8080,
    is_public BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uq_app_templates_owner_key (owner_user_id, template_key),
    KEY idx_app_templates_public (is_public, runtime),
    CONSTRAINT fk_app_templates_owner FOREIGN KEY (owner_user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS workspace_members (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    owner_user_id BIGINT UNSIGNED NOT NULL,
    member_user_id BIGINT UNSIGNED NOT NULL,
    role VARCHAR(32) NOT NULL DEFAULT 'viewer',
    status ENUM('invited','active','suspended') NOT NULL DEFAULT 'invited',
    invited_email VARCHAR(128) NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uq_workspace_member (owner_user_id, member_user_id),
    KEY idx_workspace_members_member (member_user_id, status),
    CONSTRAINT fk_workspace_members_owner FOREIGN KEY (owner_user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_workspace_members_member FOREIGN KEY (member_user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS audit_logs (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    workspace_owner_id BIGINT UNSIGNED NOT NULL,
    actor_user_id BIGINT UNSIGNED NULL,
    target_type VARCHAR(48) NOT NULL DEFAULT '',
    target_id VARCHAR(128) NOT NULL DEFAULT '',
    action VARCHAR(96) NOT NULL,
    outcome ENUM('success','denied','failure') NOT NULL DEFAULT 'success',
    request_id VARCHAR(64) NOT NULL DEFAULT '',
    ip VARCHAR(45) NOT NULL DEFAULT '',
    details JSON NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    KEY idx_audit_workspace_created (workspace_owner_id, created_at),
    KEY idx_audit_actor_action (actor_user_id, action, created_at),
    CONSTRAINT fk_audit_workspace FOREIGN KEY (workspace_owner_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_audit_actor FOREIGN KEY (actor_user_id) REFERENCES users(id) ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
