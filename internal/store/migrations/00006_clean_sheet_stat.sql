-- +goose Up
-- Whether a goalkeeper kept a clean sheet, apart from the bonus it may earn.
-- Runs recorded before this migration get false: there is no deployed data yet.
DROP VIEW current_scores;
ALTER TABLE player_scores ADD COLUMN clean_sheet boolean NOT NULL DEFAULT false;

CREATE VIEW current_scores AS
SELECT ps.player_id,
       p.name             AS player_name,
       ps.team_id,
       t.name             AS team_name,
       f.id               AS fixture_id,
       m.season_id,
       m.number           AS matchday,
       f.kickoff,
       ps.role,
       ps.minutes,
       ps.voto,
       ps.fantavoto,
       ps.senza_voto,
       ps.voto_ufficio,
       ps.breakdown,
       ps.goals,
       ps.penalties_scored,
       ps.penalties_missed,
       ps.penalties_saved,
       ps.own_goals,
       ps.assists,
       ps.yellow_cards,
       ps.sent_off,
       ps.goals_conceded,
       ps.clean_sheet,
       r.id               AS run_id,
       r.settings_sha256,
       r.voto_source,
       r.created_at       AS computed_at
FROM current_runs cr
JOIN computation_runs r ON r.id = cr.run_id
JOIN player_scores ps   ON ps.run_id = r.id
JOIN players p          ON p.id = ps.player_id
JOIN teams t            ON t.id = ps.team_id
JOIN fixtures f         ON f.id = cr.fixture_id
JOIN matchdays m        ON m.id = f.matchday_id;

-- +goose Down
DROP VIEW current_scores;
ALTER TABLE player_scores DROP COLUMN clean_sheet;

CREATE VIEW current_scores AS
SELECT ps.player_id,
       p.name             AS player_name,
       ps.team_id,
       t.name             AS team_name,
       f.id               AS fixture_id,
       m.season_id,
       m.number           AS matchday,
       f.kickoff,
       ps.role,
       ps.minutes,
       ps.voto,
       ps.fantavoto,
       ps.senza_voto,
       ps.voto_ufficio,
       ps.breakdown,
       ps.goals,
       ps.penalties_scored,
       ps.penalties_missed,
       ps.penalties_saved,
       ps.own_goals,
       ps.assists,
       ps.yellow_cards,
       ps.sent_off,
       ps.goals_conceded,
       r.id               AS run_id,
       r.settings_sha256,
       r.voto_source,
       r.created_at       AS computed_at
FROM current_runs cr
JOIN computation_runs r ON r.id = cr.run_id
JOIN player_scores ps   ON ps.run_id = r.id
JOIN players p          ON p.id = ps.player_id
JOIN teams t            ON t.id = ps.team_id
JOIN fixtures f         ON f.id = cr.fixture_id
JOIN matchdays m        ON m.id = f.matchday_id;
