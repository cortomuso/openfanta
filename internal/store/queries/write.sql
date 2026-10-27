-- name: GetRef :one
SELECT entity_id FROM provider_refs
WHERE provider = $1 AND entity = $2 AND external_id = $3;

-- name: InsertRef :one
-- Returns the winning entity_id, so concurrent inserts agree on one ID.
INSERT INTO provider_refs (provider, entity, external_id, entity_id)
VALUES ($1, $2, $3, $4)
ON CONFLICT (provider, entity, external_id) DO UPDATE SET entity_id = provider_refs.entity_id
RETURNING entity_id;

-- name: UpsertCompetition :exec
INSERT INTO competitions (id, name, country) VALUES ($1, $2, $3)
ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, country = EXCLUDED.country;

-- name: UpsertSeason :exec
INSERT INTO seasons (id, competition_id, name) VALUES ($1, $2, $3)
ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name;

-- name: UpsertMatchday :one
INSERT INTO matchdays (id, season_id, number) VALUES ($1, $2, $3)
ON CONFLICT (season_id, number) DO UPDATE SET number = EXCLUDED.number
RETURNING id;

-- name: UpsertTeam :exec
INSERT INTO teams (id, name) VALUES ($1, $2)
ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name;

-- name: UpsertPlayer :exec
INSERT INTO players (id, name) VALUES ($1, $2)
ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name;

-- name: UpsertFixture :exec
INSERT INTO fixtures (id, matchday_id, home_team_id, away_team_id, kickoff, status)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (id) DO UPDATE SET
    matchday_id = EXCLUDED.matchday_id, home_team_id = EXCLUDED.home_team_id,
    away_team_id = EXCLUDED.away_team_id, kickoff = EXCLUDED.kickoff, status = EXCLUDED.status;

-- name: InsertRawPayload :one
INSERT INTO raw_payloads (provider, request_key, sha256, http_status, body, fetched_at, last_seen_at)
VALUES ($1, $2, $3, $4, $5, $6, $6)
ON CONFLICT (provider, request_key, sha256) DO UPDATE SET last_seen_at = EXCLUDED.last_seen_at
RETURNING id, (xmax = 0)::boolean AS inserted;

-- name: LatestRawPayload :one
SELECT * FROM raw_payloads
WHERE provider = $1 AND request_key = $2
ORDER BY last_seen_at DESC, id DESC
LIMIT 1;

-- name: InsertFixtureFacts :one
INSERT INTO fixture_facts (fixture_id, facts, facts_hash, normalizer, raw_payload_ids, created_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (fixture_id, facts_hash, normalizer) DO UPDATE SET fixture_id = EXCLUDED.fixture_id
RETURNING id;

-- name: LatestFixtureFacts :one
SELECT * FROM fixture_facts WHERE fixture_id = $1 ORDER BY id DESC LIMIT 1;

-- name: GetRuleSetHash :one
SELECT content_hash FROM rule_sets WHERE id = $1 AND version = $2;

-- name: InsertRuleSet :exec
INSERT INTO rule_sets (id, version, content_hash, body, created_at) VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (id, version) DO NOTHING;

-- name: LockComputation :exec
-- Serializes run recording per (fixture, rule set, voto source) for the transaction.
SELECT pg_advisory_xact_lock(hashtextextended(sqlc.arg(key)::text, 0));

-- name: CurrentRun :one
SELECT r.id, r.input_hash FROM current_runs cr
JOIN computation_runs r ON r.id = cr.run_id
WHERE cr.fixture_id = $1 AND cr.rule_set_id = $2 AND cr.voto_source = $3;

-- name: InsertRun :exec
INSERT INTO computation_runs (
    id, fixture_id, fixture_facts_id, rule_set_id, rule_set_version, voto_source,
    engine_version, input_hash, supersedes_run_id, reason, created_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);

-- name: InsertPlayerScores :copyfrom
INSERT INTO player_scores (
    run_id, player_id, team_id, role, minutes, voto, fantavoto, senza_voto, voto_ufficio, breakdown
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10);

-- name: SetCurrentRun :exec
INSERT INTO current_runs (fixture_id, rule_set_id, voto_source, run_id) VALUES ($1, $2, $3, $4)
ON CONFLICT (fixture_id, rule_set_id, voto_source) DO UPDATE SET run_id = EXCLUDED.run_id;
