-- Persist OS, architecture and detected local capabilities for each Agent.
ALTER TABLE devices
    ADD COLUMN platform VARCHAR(32) NULL,
    ADD COLUMN architecture VARCHAR(32) NULL,
    ADD COLUMN capabilities JSON NULL;
