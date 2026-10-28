-- name: FixtureRuns :many
SELECT r.id, r.rule_set_id, r.rule_set_version, r.voto_source, r.engine_version, r.input_hash,
       r.supersedes_run_id, r.reason, r.created_at,
       EXISTS (SELECT 1 FROM current_runs cr WHERE cr.run_id = r.id) AS is_current
FROM computation_runs r
WHERE r.fixture_id = $1
ORDER BY r.created_at, r.id;

-- name: FixtureScores :many
SELECT * FROM current_scores
WHERE fixture_id = $1 AND rule_set_id = $2 AND voto_source = $3
ORDER BY team_name, player_name, player_id;

-- name: MatchdayScores :many
SELECT * FROM current_scores
WHERE season_id = $1 AND matchday = $2 AND rule_set_id = $3 AND voto_source = $4
ORDER BY kickoff, fixture_id, team_name, player_name, player_id;

-- name: PlayerSeasonScores :many
SELECT * FROM current_scores
WHERE season_id = $1 AND player_id = $2 AND rule_set_id = $3 AND voto_source = $4
ORDER BY matchday, kickoff;

-- name: ListSeasons :many
SELECT s.id, s.name, c.id AS competition_id, c.name AS competition_name, c.country AS competition_country
FROM seasons s JOIN competitions c ON c.id = s.competition_id
ORDER BY c.name, s.name DESC;

-- name: GetSeason :one
SELECT s.id, s.name, c.id AS competition_id, c.name AS competition_name, c.country AS competition_country
FROM seasons s JOIN competitions c ON c.id = s.competition_id
WHERE s.id = $1;

-- name: SeasonFixtures :many
SELECT m.number AS matchday, f.id, f.kickoff, f.status,
       h.id AS home_id, h.name AS home_name, a.id AS away_id, a.name AS away_name
FROM matchdays m
JOIN fixtures f ON f.matchday_id = m.id
JOIN teams h ON h.id = f.home_team_id
JOIN teams a ON a.id = f.away_team_id
WHERE m.season_id = $1
ORDER BY m.number, f.kickoff, f.id;

-- name: MatchdayExists :one
SELECT EXISTS (SELECT 1 FROM matchdays WHERE season_id = $1 AND number = $2);

-- name: GetPlayer :one
SELECT id, name FROM players WHERE id = $1;

-- name: GetFixture :one
SELECT f.id, f.kickoff, f.status, m.number AS matchday, m.season_id,
       h.id AS home_id, h.name AS home_name, a.id AS away_id, a.name AS away_name
FROM fixtures f
JOIN matchdays m ON m.id = f.matchday_id
JOIN teams h ON h.id = f.home_team_id
JOIN teams a ON a.id = f.away_team_id
WHERE f.id = $1;

-- name: ListRuleSets :many
SELECT id, version, content_hash, body, created_at FROM rule_sets ORDER BY id, created_at;

-- name: GetRuleSet :one
SELECT id, version, content_hash, body, created_at FROM rule_sets WHERE id = $1 AND version = $2;

-- name: SettingsHead :one
SELECT * FROM settings_versions WHERE league_id = $1 ORDER BY seq DESC LIMIT 1;

-- name: ListSettingsVersions :many
SELECT * FROM settings_versions WHERE league_id = $1 ORDER BY seq;

-- name: GetSettingsVersion :one
SELECT * FROM settings_versions WHERE league_id = $1 AND seq = $2;

-- name: HasClosedMatchdayFrom :one
-- True when matchday $2 or any later one of the season is already closed.
SELECT EXISTS (
    SELECT 1 FROM matchdays WHERE season_id = $1 AND number >= $2 AND closed_at IS NOT NULL
);
