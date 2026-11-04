-- +goose Up
-- The provider's score and how far the match has got. The score is both sides or neither; the
-- period is empty when the provider's state says nothing about it.
ALTER TABLE fixtures
    ADD COLUMN home_score smallint CHECK (home_score >= 0),
    ADD COLUMN away_score smallint CHECK (away_score >= 0),
    ADD COLUMN period text NOT NULL DEFAULT ''
        CHECK (period IN ('', 'not_started', 'first_half', 'half_time', 'second_half', 'extra_time', 'penalties', 'finished')),
    ADD CONSTRAINT fixtures_score_pair CHECK ((home_score IS NULL) = (away_score IS NULL));

-- +goose Down
ALTER TABLE fixtures
    DROP CONSTRAINT fixtures_score_pair,
    DROP COLUMN period, DROP COLUMN away_score, DROP COLUMN home_score;
