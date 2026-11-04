-- +goose Up
-- A player's season aggregates over the current runs, one row per settings document and voto
-- source. Averages are in hundredths, rounded half up, like every score. Games with voto count
-- rows that have a voto; appearances count rows with minutes on the pitch.
CREATE VIEW player_season_stats AS
SELECT player_id,
       season_id,
       settings_sha256,
       voto_source,
       (COUNT(*) FILTER (WHERE minutes > 0))::int       AS appearances,
       COALESCE(SUM(minutes), 0)::int                   AS minutes,
       COUNT(voto)::int                                 AS games_with_voto,
       ROUND(AVG(voto))::int                            AS avg_voto,
       ROUND(AVG(fantavoto))::int                       AS avg_fantavoto,
       COALESCE(SUM(goals), 0)::int                     AS goals,
       COALESCE(SUM(penalties_scored), 0)::int          AS penalties_scored,
       COALESCE(SUM(penalties_missed), 0)::int          AS penalties_missed,
       COALESCE(SUM(penalties_saved), 0)::int           AS penalties_saved,
       COALESCE(SUM(own_goals), 0)::int                 AS own_goals,
       COALESCE(SUM(assists), 0)::int                   AS assists,
       COALESCE(SUM(assists_set_piece), 0)::int         AS assists_set_piece,
       COALESCE(SUM(assists_contribution), 0)::int      AS assists_contribution,
       COALESCE(SUM(yellow_cards), 0)::int              AS yellow_cards,
       (COUNT(*) FILTER (WHERE sent_off))::int          AS sent_off,
       COALESCE(SUM(goals_conceded), 0)::int            AS goals_conceded,
       (COUNT(*) FILTER (WHERE clean_sheet))::int       AS clean_sheets,
       COALESCE(SUM(decisive_goals_win), 0)::int        AS decisive_goals_win,
       COALESCE(SUM(decisive_goals_draw), 0)::int       AS decisive_goals_draw,
       (COUNT(*) FILTER (WHERE player_of_the_match))::int AS player_of_the_match
FROM current_scores
GROUP BY player_id, season_id, settings_sha256, voto_source;

-- +goose Down
DROP VIEW player_season_stats;
