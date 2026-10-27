-- +goose Up
-- Flattened view of the scores currently served, one row per player per fixture
-- per (rule set family, voto source).
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
