-- +goose Up
-- Birth year as the provider gives it; null when unknown.
ALTER TABLE players ADD COLUMN birth_year smallint CHECK (birth_year BETWEEN 1900 AND 2100);

-- The squad a team lists in a season: a player's club, role and whether they are
-- still listed. One row per player and season, so it is also the club history.
CREATE TABLE player_seasons (
    season_id  uuid NOT NULL REFERENCES seasons (id),
    player_id  uuid NOT NULL REFERENCES players (id),
    team_id    uuid NOT NULL REFERENCES teams (id),
    role       text CHECK (role IN ('P', 'D', 'C', 'A')),
    listed     boolean NOT NULL,
    PRIMARY KEY (season_id, player_id)
);
CREATE INDEX player_seasons_team ON player_seasons (season_id, team_id);

-- +goose Down
DROP TABLE player_seasons;
ALTER TABLE players DROP COLUMN birth_year;
