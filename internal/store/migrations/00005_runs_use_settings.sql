-- +goose Up

-- Runs are identified by the sha256 of the settings document that scored them,
-- not by a rule set id and version. There is no deployed data yet, so existing
-- runs keep an empty hash instead of being converted.
DROP VIEW current_scores;

ALTER TABLE current_runs DROP CONSTRAINT current_runs_pkey;
ALTER TABLE current_runs DROP COLUMN rule_set_id;
ALTER TABLE current_runs ADD COLUMN settings_sha256 text NOT NULL DEFAULT '';
ALTER TABLE current_runs ALTER COLUMN settings_sha256 DROP DEFAULT;
ALTER TABLE current_runs ADD PRIMARY KEY (fixture_id, settings_sha256, voto_source);

ALTER TABLE computation_runs DROP CONSTRAINT computation_runs_rule_set_id_rule_set_version_fkey;
ALTER TABLE computation_runs DROP COLUMN rule_set_id;
ALTER TABLE computation_runs DROP COLUMN rule_set_version;
ALTER TABLE computation_runs ADD COLUMN settings_sha256 text NOT NULL DEFAULT '';
ALTER TABLE computation_runs ALTER COLUMN settings_sha256 DROP DEFAULT;

DROP TABLE rule_sets;

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

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
    RAISE EXCEPTION 'migration 00005 is not reversible: rule sets and run links are dropped';
END $$;
-- +goose StatementEnd
