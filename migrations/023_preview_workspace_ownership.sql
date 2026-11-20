-- Add creator attribution for workspace-owned preview deployments.
-- Existing preview rows are preserved; the new nullable creator field is not backfilled.

ALTER TABLE preview_deployments
    ADD COLUMN created_by_user_id BIGINT UNSIGNED NULL AFTER user_id,
    ADD KEY idx_previews_creator (created_by_user_id),
    ADD CONSTRAINT fk_previews_creator FOREIGN KEY (created_by_user_id) REFERENCES users(id) ON DELETE SET NULL;
