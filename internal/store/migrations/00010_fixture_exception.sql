-- +goose Up
-- Whether a real match was postponed or suspended, for the real-match policy.
ALTER TABLE fixtures
    ADD COLUMN exception text NOT NULL DEFAULT '' CHECK (exception IN ('', 'postponed', 'suspended'));

-- +goose Down
ALTER TABLE fixtures DROP COLUMN exception;
