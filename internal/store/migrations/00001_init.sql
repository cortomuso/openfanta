-- +goose Up

-- Reference entities. IDs are OpenFanta's own UUIDv7s; provider IDs live in provider_refs.
CREATE TABLE competitions (
    id      uuid PRIMARY KEY,
    name    text NOT NULL,
    country text NOT NULL DEFAULT ''
);

CREATE TABLE seasons (
    id             uuid PRIMARY KEY,
    competition_id uuid NOT NULL REFERENCES competitions (id),
    name           text NOT NULL
);

CREATE TABLE matchdays (
    id        uuid PRIMARY KEY,
    season_id uuid NOT NULL REFERENCES seasons (id),
    number    integer NOT NULL CHECK (number > 0),
    UNIQUE (season_id, number)
);

CREATE TABLE teams (
    id   uuid PRIMARY KEY,
    name text NOT NULL
);

CREATE TABLE players (
    id   uuid PRIMARY KEY,
    name text NOT NULL
);

CREATE TABLE fixtures (
    id           uuid PRIMARY KEY,
    matchday_id  uuid NOT NULL REFERENCES matchdays (id),
    home_team_id uuid NOT NULL REFERENCES teams (id),
    away_team_id uuid NOT NULL REFERENCES teams (id),
    kickoff      timestamptz,
    status       text NOT NULL
);
CREATE INDEX fixtures_matchday_idx ON fixtures (matchday_id);

-- Maps a provider's external ID to an OpenFanta entity.
CREATE TABLE provider_refs (
    provider    text NOT NULL,
    entity      text NOT NULL CHECK (entity IN ('competition', 'season', 'matchday', 'team', 'player', 'fixture')),
    external_id text NOT NULL,
    entity_id   uuid NOT NULL,
    PRIMARY KEY (provider, entity, external_id)
);

-- Append-only raw upstream responses. A changed response is a new row.
CREATE TABLE raw_payloads (
    id           bigserial PRIMARY KEY,
    provider     text NOT NULL,
    request_key  text NOT NULL,
    sha256       text NOT NULL,
    http_status  integer NOT NULL,
    body         jsonb NOT NULL,
    fetched_at   timestamptz NOT NULL,
    last_seen_at timestamptz NOT NULL,
    UNIQUE (provider, request_key, sha256)
);
CREATE INDEX raw_payloads_lookup_idx ON raw_payloads (provider, request_key, last_seen_at DESC);

-- Normalized, provider-agnostic facts derived from raw payloads.
CREATE TABLE fixture_facts (
    id                 bigserial PRIMARY KEY,
    fixture_id         uuid NOT NULL REFERENCES fixtures (id),
    facts              jsonb NOT NULL,
    facts_hash         text NOT NULL,
    normalizer         text NOT NULL,
    raw_payload_ids    bigint[] NOT NULL,
    created_at         timestamptz NOT NULL,
    UNIQUE (fixture_id, facts_hash, normalizer)
);

-- Rule sets as loaded. A published (id, version) never changes.
CREATE TABLE rule_sets (
    id           text NOT NULL,
    version      text NOT NULL,
    content_hash text NOT NULL,
    body         jsonb NOT NULL,
    created_at   timestamptz NOT NULL,
    PRIMARY KEY (id, version)
);

-- One immutable computation of a fixture under a rule set and voto source.
CREATE TABLE computation_runs (
    id                uuid PRIMARY KEY,
    fixture_id        uuid NOT NULL REFERENCES fixtures (id),
    fixture_facts_id  bigint NOT NULL REFERENCES fixture_facts (id),
    rule_set_id       text NOT NULL,
    rule_set_version  text NOT NULL,
    voto_source       text NOT NULL,
    engine_version    text NOT NULL,
    input_hash        text NOT NULL,
    supersedes_run_id uuid REFERENCES computation_runs (id),
    reason            text NOT NULL,
    created_at        timestamptz NOT NULL,
    FOREIGN KEY (rule_set_id, rule_set_version) REFERENCES rule_sets (id, version)
);
CREATE INDEX computation_runs_fixture_idx ON computation_runs (fixture_id, created_at);

-- Points are integer hundredths (650 = 6.5).
CREATE TABLE player_scores (
    run_id       uuid NOT NULL REFERENCES computation_runs (id),
    player_id    uuid NOT NULL REFERENCES players (id),
    team_id      uuid NOT NULL REFERENCES teams (id),
    role         text NOT NULL,
    minutes      integer NOT NULL,
    voto         integer,
    fantavoto    integer,
    senza_voto   boolean NOT NULL,
    voto_ufficio boolean NOT NULL,
    breakdown    jsonb NOT NULL,
    PRIMARY KEY (run_id, player_id)
);
CREATE INDEX player_scores_player_idx ON player_scores (player_id);

-- The run currently served for each (fixture, rule set family, voto source).
CREATE TABLE current_runs (
    fixture_id  uuid NOT NULL REFERENCES fixtures (id),
    rule_set_id text NOT NULL,
    voto_source text NOT NULL,
    run_id      uuid NOT NULL REFERENCES computation_runs (id),
    PRIMARY KEY (fixture_id, rule_set_id, voto_source)
);

-- +goose Down
DROP TABLE current_runs;
DROP TABLE player_scores;
DROP TABLE computation_runs;
DROP TABLE rule_sets;
DROP TABLE fixture_facts;
DROP TABLE raw_payloads;
DROP TABLE provider_refs;
DROP TABLE fixtures;
DROP TABLE players;
DROP TABLE teams;
DROP TABLE matchdays;
DROP TABLE seasons;
DROP TABLE competitions;
