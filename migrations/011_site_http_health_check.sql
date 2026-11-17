-- Optional HTTP health check path for immutable deployments.
ALTER TABLE sites
    ADD COLUMN health_check_path VARCHAR(255) NOT NULL DEFAULT '';
