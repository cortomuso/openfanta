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
SELECT m.number AS matchday, f.id, f.kickoff, f.status, f.exception, f.home_score, f.away_score, f.period,
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
SELECT f.id, f.kickoff, f.status, f.exception, f.home_score, f.away_score, f.period, m.number AS matchday, m.season_id,
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

-- name: ListClubs :many
-- Clubs are the teams of the competitions; one that never listed a squad still shows up.
SELECT id, name FROM teams ORDER BY name, id;

-- name: GetClub :one
SELECT id, name FROM teams WHERE id = $1;

-- name: ClubLatestSeason :one
-- The latest season in which the club has a squad; no row for a club without one.
SELECT s.id, s.name, c.id AS competition_id, c.name AS competition_name, c.country AS competition_country
FROM player_seasons x
JOIN seasons s ON s.id = x.season_id
JOIN competitions c ON c.id = s.competition_id
WHERE x.team_id = $1
ORDER BY s.name DESC, s.id DESC
LIMIT 1;

-- name: ClubSquad :many
SELECT p.id AS player_id, p.name AS player_name, p.birth_year, x.role, x.listed
FROM player_seasons x
JOIN players p ON p.id = x.player_id
WHERE x.team_id = $1 AND x.season_id = $2
ORDER BY p.name, p.id;

-- name: PlayerClubHistory :many
SELECT x.season_id, s.name AS season_name, x.team_id AS club_id, t.name AS club_name, x.role, x.listed
FROM player_seasons x
JOIN seasons s ON s.id = x.season_id
JOIN teams t ON t.id = x.team_id
WHERE x.player_id = $1
ORDER BY s.name DESC, s.id DESC;

-- name: ListPlayers :many
-- Players with the club, role and status of one season: the requested one, else each player's
-- latest. A requested season also keeps only players listed in it. The statistics are those of
-- that same season under the given settings and voto source. Order: the sort value (an average,
-- highest first, players without one last; NULL throughout for the name sort), then name, then id.
-- The after_* values are the last row of the previous page; after_name is NULL on the first page
-- and after_sort is NULL when that last row had no sort value.
WITH entries AS (
    SELECT p.id, p.name, p.birth_year, ps.season_id, ps.team_id AS club_id, t.name AS club_name, ps.role, ps.listed
    FROM players p
    LEFT JOIN player_seasons ps ON ps.player_id = p.id AND ps.season_id = COALESCE(
        sqlc.narg(season_id)::uuid,
        (SELECT x.season_id FROM player_seasons x JOIN seasons s ON s.id = x.season_id
         WHERE x.player_id = p.id ORDER BY s.name DESC, s.id DESC LIMIT 1))
    LEFT JOIN teams t ON t.id = ps.team_id
    WHERE (sqlc.narg(name_pattern)::text IS NULL OR p.name ILIKE sqlc.narg(name_pattern)::text ESCAPE '\')
      AND (sqlc.narg(role)::text IS NULL OR ps.role = sqlc.narg(role)::text)
      AND (sqlc.narg(club_id)::uuid IS NULL OR ps.team_id = sqlc.narg(club_id)::uuid)
      AND (sqlc.narg(season_id)::uuid IS NULL OR ps.player_id IS NOT NULL)
      AND (sqlc.narg(birth_from)::int IS NULL OR p.birth_year >= sqlc.narg(birth_from)::int)
      AND (sqlc.narg(birth_to)::int IS NULL OR p.birth_year <= sqlc.narg(birth_to)::int)
), ranked AS (
    SELECT e.*, st.appearances, st.minutes, st.games_with_voto, st.avg_voto, st.avg_fantavoto,
           st.goals, st.penalties_scored, st.penalties_missed, st.penalties_saved, st.own_goals,
           st.assists, st.assists_set_piece, st.assists_contribution, st.yellow_cards, st.sent_off,
           st.goals_conceded, st.clean_sheets, st.decisive_goals_win, st.decisive_goals_draw, st.player_of_the_match,
           (CASE sqlc.arg(sort)::text WHEN 'avg_voto' THEN st.avg_voto WHEN 'avg_fantavoto' THEN st.avg_fantavoto END)::int AS sort_value
    FROM entries e
    LEFT JOIN player_season_stats st ON st.player_id = e.id AND st.season_id = e.season_id
         AND st.settings_sha256 = sqlc.arg(settings_sha256)::text AND st.voto_source = sqlc.arg(voto_source)::text
)
SELECT r.id, r.name, r.birth_year, r.season_id, r.club_id, r.club_name, r.role, r.listed, r.appearances, r.minutes, r.games_with_voto, r.avg_voto, r.avg_fantavoto, r.goals, r.penalties_scored, r.penalties_missed, r.penalties_saved, r.own_goals, r.assists, r.assists_set_piece, r.assists_contribution, r.yellow_cards, r.sent_off, r.goals_conceded, r.clean_sheets, r.decisive_goals_win, r.decisive_goals_draw, r.player_of_the_match
FROM ranked r
WHERE sqlc.narg(after_name)::text IS NULL
   OR CASE WHEN sqlc.narg(after_sort)::int IS NULL
           THEN r.sort_value IS NULL AND (r.name, r.id) > (sqlc.narg(after_name)::text, sqlc.narg(after_id)::uuid)
           ELSE r.sort_value IS NULL OR r.sort_value < sqlc.narg(after_sort)::int
                OR (r.sort_value = sqlc.narg(after_sort)::int AND (r.name, r.id) > (sqlc.narg(after_name)::text, sqlc.narg(after_id)::uuid))
      END
ORDER BY r.sort_value DESC NULLS LAST, r.name, r.id
LIMIT sqlc.arg(row_limit)::int;

-- name: GetPlayerSeasonStats :one
SELECT * FROM player_season_stats
WHERE player_id = $1 AND season_id = $2 AND settings_sha256 = $3 AND voto_source = $4;
