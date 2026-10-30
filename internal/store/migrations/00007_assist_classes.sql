-- +goose Up
-- Assists by class. The existing assists column keeps counting every class.
DROP VIEW current_scores;
ALTER TABLE player_scores
    ADD COLUMN assists_set_piece    integer NOT NULL DEFAULT 0,
    ADD COLUMN assists_contribution integer NOT NULL DEFAULT 0;

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
       ps.assists_set_piece,
       ps.assists_contribution,
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
ALTER TABLE player_scores DROP COLUMN assists_set_piece, DROP COLUMN assists_contribution;

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
