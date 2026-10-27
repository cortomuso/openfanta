# Sportmonks test fixtures

**These payloads are synthetic.** They follow the documented Sportmonks Football API v3
response shape (field names, type IDs, includes), but they were written by hand because
no API token was available when the parser was built. Team and player names are invented.

| File | Request it stands for |
|---|---|
| `season_777.json` | `GET /seasons/777?include=league.country` |
| `schedule_777.json` | `GET /schedules/seasons/777` |
| `fixture_5001_ft.json` | `GET /fixtures/5001?include=participants;lineups.details;events;state` (finished) |
| `fixture_5001_live.json` | The same fixture while in play |
| `fixture_5001_ft.golden.json` | Expected `ParseFixture` output; regenerate with `go test ./internal/provider/sportmonks -update` |

The finished fixture deliberately covers: a goal with an assist, a scored penalty, a missed
penalty, a saved penalty (`sub_type_id` 1509), an own goal, substitutions (where `player_id`
is the player coming on), a goalkeeper change, a yellow card, a second yellow, a straight red,
a card shown to a coach, a goal rescinded by VAR, events listed out of order, and a rating
given as a string.

Replace or supplement them with recorded real responses once a token is available. Raw
responses are stored verbatim in the `raw_payloads` table, so they can be exported from there.
Never commit an API token.
