-- name: FixtureRuns :many
SELECT r.id, r.rule_set_id, r.rule_set_version, r.voto_source, r.engine_version, r.input_hash,
       r.supersedes_run_id, r.reason, r.created_at,
       EXISTS (SELECT 1 FROM current_runs cr WHERE cr.run_id = r.id) AS is_current
FROM computation_runs r
WHERE r.fixture_id = $1
ORDER BY r.created_at, r.id;
