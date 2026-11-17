-- Per-site bounded deployment timeouts. Existing sites keep safe defaults.
ALTER TABLE sites
    ADD COLUMN clone_timeout_seconds INT NOT NULL DEFAULT 600,
    ADD COLUMN build_timeout_seconds INT NOT NULL DEFAULT 1200,
    ADD COLUMN health_check_timeout_seconds INT NOT NULL DEFAULT 15;
