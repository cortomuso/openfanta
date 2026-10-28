-- +goose Up

-- A league plays one season. Its scoring is described by its settings versions.
CREATE TABLE leagues (
    id         uuid PRIMARY KEY,
    season_id  uuid NOT NULL REFERENCES seasons (id),
    name       text NOT NULL,
    created_at timestamptz NOT NULL
);

-- A closed matchday is final: settings can no longer take effect from it.
ALTER TABLE matchdays ADD COLUMN closed_at timestamptz;

-- Append-only history of a league's settings. canonical_json is text, not
-- jsonb, so the exact canonical bytes survive; sha256 covers those bytes and
-- prev_sha256 links each version to the one before it.
CREATE TABLE settings_versions (
    league_id               uuid NOT NULL REFERENCES leagues (id),
    seq                     integer NOT NULL CHECK (seq > 0),
    canonical_json          text NOT NULL,
    sha256                  text NOT NULL,
    prev_sha256             text,
    schema                  text NOT NULL,
    engine                  text NOT NULL,
    effective_from_matchday integer NOT NULL CHECK (effective_from_matchday > 0),
    author                  text NOT NULL,
    reason                  text NOT NULL,
    created_at              timestamptz NOT NULL,
    PRIMARY KEY (league_id, seq),
    CHECK ((seq = 1) = (prev_sha256 IS NULL))
);

-- +goose StatementBegin
CREATE FUNCTION settings_versions_append_only() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'settings_versions is append-only';
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER settings_versions_append_only
    BEFORE UPDATE OR DELETE ON settings_versions
    FOR EACH ROW EXECUTE FUNCTION settings_versions_append_only();

-- +goose Down
DROP TRIGGER settings_versions_append_only ON settings_versions;
DROP FUNCTION settings_versions_append_only();
DROP TABLE settings_versions;
ALTER TABLE matchdays DROP COLUMN closed_at;
DROP TABLE leagues;
