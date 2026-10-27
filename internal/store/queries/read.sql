-- name: FixtureRuns :many
SELECT r.id, r.rule_set_id, r.rule_set_version, r.voto_source, r.engine_version, r.input_hash,
       r.supersedes_run_id, r.reason, r.created_at,
       EXISTS (SELECT 1 FROM current_runs cr WHERE cr.run_id = r.id) AS is_current
FROM computation_runs r
WHERE r.fixture_id = $1
ORDER BY r.created_at, r.id;

-- name: FixtureScores :many
-- Current scores for one fixture under a rule set family and voto source.
SELECT ps.player_id, p.name AS player_name, ps.team_id, t.name AS team_name, ps.role, ps.minutes,
       ps.voto, ps.fantavoto, ps.senza_voto, ps.voto_ufficio, ps.breakdown,
       r.id AS run_id, r.rule_set_id, r.rule_set_version, r.voto_source, r.created_at AS computed_at
FROM current_runs cr
JOIN computation_runs r ON r.id = cr.run_id
JOIN player_scores ps ON ps.run_id = r.id
JOIN players p ON p.id = ps.player_id
JOIN teams t ON t.id = ps.team_id
WHERE cr.fixture_id = $1 AND cr.rule_set_id = $2 AND cr.voto_source = $3
ORDER BY t.name, p.name, ps.player_id;
