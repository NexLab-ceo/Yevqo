ALTER TABLE database_servers
    ADD COLUMN storage_limit_gb VARCHAR(16) NOT NULL DEFAULT 'unlimited',
    ADD COLUMN ram_budget_mb INT NOT NULL DEFAULT 0,
    ADD COLUMN max_connections INT NOT NULL DEFAULT 100,
    ADD COLUMN monitor_storage BOOLEAN NOT NULL DEFAULT TRUE,
    ADD COLUMN auto_start BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN process_policy ENUM('balanced','persistent','manual') NOT NULL DEFAULT 'balanced',
    ADD COLUMN bind_address VARCHAR(45) NOT NULL DEFAULT '127.0.0.1',
    ADD COLUMN ssl_mode ENUM('require','prefer','disable') NOT NULL DEFAULT 'require',
    ADD COLUMN auth_method VARCHAR(32) NOT NULL DEFAULT 'password',
    ADD COLUMN remote_access BOOLEAN NOT NULL DEFAULT FALSE;
