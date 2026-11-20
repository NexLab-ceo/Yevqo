-- Add environment-scoped database-instance membership.
-- Existing database instances are preserved. Only unambiguous legacy mappings are backfilled.

ALTER TABLE project_environments
    ADD UNIQUE KEY uq_project_environments_project_id_id (project_id, id);

CREATE TABLE IF NOT EXISTS project_environment_databases (
    project_id BIGINT UNSIGNED NOT NULL,
    environment_id BIGINT UNSIGNED NOT NULL,
    database_id BIGINT UNSIGNED NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (project_id, database_id),
    KEY idx_project_environment_databases_environment (project_id, environment_id),
    CONSTRAINT fk_project_environment_databases_environment
        FOREIGN KEY (project_id, environment_id)
        REFERENCES project_environments(project_id, id) ON DELETE CASCADE,
    CONSTRAINT fk_project_environment_databases_database
        FOREIGN KEY (database_id) REFERENCES db_instances(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

INSERT IGNORE INTO project_environment_databases (project_id, environment_id, database_id)
SELECT pds.project_id, MIN(eds.environment_id), d.id
FROM project_database_servers pds
JOIN project_environment_database_servers eds ON eds.project_id=pds.project_id AND eds.database_server_id=pds.database_server_id
JOIN db_instances d ON d.server_id=pds.database_server_id
JOIN projects p ON p.id=pds.project_id AND p.user_id=d.user_id
GROUP BY pds.project_id, d.id
HAVING COUNT(DISTINCT eds.environment_id) = 1;
