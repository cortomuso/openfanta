-- +goose Up
-- The version of the voto source's parameters, so a tuned model is auditable.
ALTER TABLE computation_runs ADD COLUMN voto_source_version text NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE computation_runs DROP COLUMN voto_source_version;
