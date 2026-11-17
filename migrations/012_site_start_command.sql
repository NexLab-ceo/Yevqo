-- Optional per-site custom process command. Commands are executed without a shell.
ALTER TABLE sites
    ADD COLUMN start_command VARCHAR(512) NOT NULL DEFAULT '';
