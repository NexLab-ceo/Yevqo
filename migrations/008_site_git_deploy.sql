-- Git-backed website deployment metadata. Additive-only; existing site rows and files are preserved.
ALTER TABLE sites
    ADD COLUMN git_url VARCHAR(512) NULL,
    ADD COLUMN git_branch VARCHAR(128) NOT NULL DEFAULT 'main',
    ADD COLUMN build_command VARCHAR(512) NOT NULL DEFAULT '',
    ADD COLUMN root_dir VARCHAR(255) NOT NULL DEFAULT '.',
    ADD COLUMN git_token_enc BLOB NULL,
    ADD COLUMN deployed_commit CHAR(40) NULL,
    ADD COLUMN deployed_at TIMESTAMP NULL;
