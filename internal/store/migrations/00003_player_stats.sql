-- +goose Up
-- Per-player match facts behind each score, so members can audit a fantavoto.
-- Runs recorded before this migration get zeros: there is no deployed data yet.
ALTER TABLE player_scores
    ADD COLUMN goals            integer NOT NULL DEFAULT 0,
    ADD COLUMN penalties_scored integer NOT NULL DEFAULT 0,
    ADD COLUMN penalties_missed integer NOT NULL DEFAULT 0,
    ADD COLUMN penalties_saved  integer NOT NULL DEFAULT 0,
    ADD COLUMN own_goals        integer NOT NULL DEFAULT 0,
    ADD COLUMN assists          integer NOT NULL DEFAULT 0,
    ADD COLUMN yellow_cards     integer NOT NULL DEFAULT 0,
    ADD COLUMN sent_off         boolean NOT NULL DEFAULT false,
    ADD COLUMN goals_conceded   integer NOT NULL DEFAULT 0;

DROP VIEW current_scores;
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
       r.rule_set_id,
       r.rule_set_version,
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
CREATE VIEW current_scores AS
SELECT ps.player_id, p.name AS player_name, ps.team_id, t.name AS team_name, f.id AS fixture_id,
       m.season_id, m.number AS matchday, f.kickoff, ps.role, ps.minutes, ps.voto, ps.fantavoto,
       ps.senza_voto, ps.voto_ufficio, ps.breakdown, r.id AS run_id, r.rule_set_id,
       r.rule_set_version, r.voto_source, r.created_at AS computed_at
FROM current_runs cr
JOIN computation_runs r ON r.id = cr.run_id
JOIN player_scores ps   ON ps.run_id = r.id
JOIN players p          ON p.id = ps.player_id
JOIN teams t            ON t.id = ps.team_id
JOIN fixtures f         ON f.id = cr.fixture_id
JOIN matchdays m        ON m.id = f.matchday_id;
ALTER TABLE player_scores
    DROP COLUMN goals, DROP COLUMN penalties_scored, DROP COLUMN penalties_missed,
    DROP COLUMN penalties_saved, DROP COLUMN own_goals, DROP COLUMN assists,
    DROP COLUMN yellow_cards, DROP COLUMN sent_off, DROP COLUMN goals_conceded;
