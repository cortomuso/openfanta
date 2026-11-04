-- name: FixtureRuns :many
SELECT r.id, r.settings_sha256, r.voto_source, r.voto_source_version, r.engine_version, r.input_hash,
       r.supersedes_run_id, r.reason, r.created_at,
       EXISTS (SELECT 1 FROM current_runs cr WHERE cr.run_id = r.id) AS is_current
FROM computation_runs r
WHERE r.fixture_id = $1
ORDER BY r.created_at, r.id;

-- name: FixtureScores :many
SELECT * FROM current_scores
WHERE fixture_id = $1 AND settings_sha256 = $2 AND voto_source = $3
ORDER BY team_name, player_name, player_id;

-- name: MatchdayScores :many
SELECT * FROM current_scores
WHERE season_id = $1 AND matchday = $2 AND settings_sha256 = $3 AND voto_source = $4
ORDER BY kickoff, fixture_id, team_name, player_name, player_id;

-- name: PlayerSeasonScores :many
SELECT * FROM current_scores
WHERE season_id = $1 AND player_id = $2 AND settings_sha256 = $3 AND voto_source = $4
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
SELECT m.number AS matchday, f.id, f.kickoff, f.status, f.exception,
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
SELECT id, name, birth_year FROM players WHERE id = $1;

-- name: GetPlayerLatestSquadEntry :one
-- The club, role and status of a player's latest season in the squads; no row for a player no
-- squad ever named.
SELECT x.team_id AS club_id, t.name AS club_name, x.role, x.listed
FROM player_seasons x
JOIN seasons s ON s.id = x.season_id
JOIN teams t ON t.id = x.team_id
WHERE x.player_id = $1
ORDER BY s.name DESC, s.id DESC
LIMIT 1;

-- name: SeasonTeamRefs :many
-- The provider refs of the teams that play in a season, for fetching their squads.
SELECT DISTINCT r.external_id, t.id, t.name
FROM matchdays m
JOIN fixtures f ON f.matchday_id = m.id
JOIN teams t ON t.id IN (f.home_team_id, f.away_team_id)
JOIN provider_refs r ON r.entity = 'team' AND r.entity_id = t.id AND r.provider = $2
WHERE m.season_id = $1
ORDER BY r.external_id;

-- name: GetFixture :one
SELECT f.id, f.kickoff, f.status, f.exception, m.number AS matchday, m.season_id,
       h.id AS home_id, h.name AS home_name, a.id AS away_id, a.name AS away_name
FROM fixtures f
JOIN matchdays m ON m.id = f.matchday_id
JOIN teams h ON h.id = f.home_team_id
JOIN teams a ON a.id = f.away_team_id
WHERE f.id = $1;

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
