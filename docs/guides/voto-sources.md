---
title: Voto sources
description: Where the base voto comes from, what each source does, and how to change the openfanta-stat parameters.
---

Every finished fixture is scored with each configured voto source, and each gets its own run. A league's settings
(`voto.source`, see [League settings](../settings.md#voto-source-and-rounding)) say which run it reads, and
`voto.rounding` is applied on top. `GET /v1/voto-sources` lists the sources with their version.

| Source | Where the voto comes from |
|---|---|
| `provider-rating` | The Sportmonks player rating (detail type 118), unchanged at two decimals. |
| `openfanta-stat` | OpenFanta's own rating from match events, per-player statistics and the team result. Independent of the provider's rating. |
| `average` | The mean of its children (`OPENFANTA_VOTO_AVERAGE_OF`, default `provider-rating` and `openfanta-stat`), rounded half away from zero. A player missing from any child has no voto, so the league's `senza_voto` rules apply. |

Configure what is computed with `OPENFANTA_VOTO_SOURCES` (comma separated, default all three). `average` cannot include itself.
No Sportmonks add-ons are used: only what `lineups.details` returns on the standard plan.

## How openfanta-stat works

For every player who took the pitch **and** has a statistics block:

```
delta = sum(weight[position][term] x count)      events, stats and the team result
delta = delta x m / (m + 15)                      m = minutes played; short appearances move less
delta = curve(delta)                              identity in v1
voto  = clamp(600 + delta, 300, 1000)             hundredths
```

Everything is an integer in hundredths and every division rounds half away from zero, so the same facts give the
same voti. Goals, assists, cards, penalties, own goals, goals conceded and minutes come from the match events;
the provider's duplicates of those are never used. Goals and assists therefore count both here and in the
fantavoto bonus, as they do in human voti.

- **Position** is the provider's match position (goalkeeper, defender, midfielder, attacker), not the listone role.
  An unknown position uses the midfielder column, or the goalkeeper one for a goalkeeper.
- **Missing stats.** Sportmonks never sends a stat whose value is 0. For a player with a statistics block an absent stat is 0;
  a player with no block at all (a bench player who never played) is unknown and gets no voto.
- **Derived counts:** inaccurate passes = passes - accurate passes, failed dribbles = dribbles - dribbles won,
  duels lost = duels - duels won. `key_pass` uses key passes, or chances created when there are no key passes, never both.
- **Team result:** a win is +20 and a loss -20; goal difference adds 10 per goal (cap 30) for defenders and midfielders,
  5 per goal (cap 15) for attackers, nothing for goalkeepers. The clean sheet term needs 60 minutes on the pitch and no goal conceded.

The weights are engineering priors, not published constants. Weights per unit, in hundredths:

| Term | DEF | MID | FWD | GK |
|---|---|---|---|---|
| `goal` | 140 | 120 | 100 | 150 |
| `assist` | 50 | 50 | 50 | 50 |
| `penalty_missed` | -70 | -70 | -70 | -70 |
| `own_goal` | -80 | -80 | -80 | -80 |
| `yellow` | -30 | -30 | -30 | -30 |
| `sent_off` | -150 | -150 | -150 | -150 |
| `penalty_saved` |  |  |  | 100 |
| `goal_conceded` | -10 | 0 | 0 | -40 |
| `clean_sheet` | 20 | 0 | 0 | 30 |
| `key_pass` | 15 | 15 | 15 | 15 |
| `shot_on_target` | 10 | 10 | 10 | 0 |
| `shot_off_target` | -2 | -2 | -2 | 0 |
| `accurate_pass` | 1 | 1 | 1 | 0 |
| `inaccurate_pass` | -2 | -2 | -2 | -1 |
| `dribble_won` | 8 | 8 | 8 | 0 |
| `failed_dribble` | -3 | -3 | -3 | 0 |
| `dispossessed` | -5 | -5 | -5 | -5 |
| `duel_won` | 3 | 3 | 3 | 0 |
| `duel_lost` | -3 | -3 | -3 | 0 |
| `aerial_won` | 4 | 3 | 3 | 0 |
| `aerial_lost` | -2 | -2 | -2 | 0 |
| `tackle_won` | 10 | 10 | 10 | 0 |
| `interception` | 12 | 10 | 10 | 5 |
| `clearance` | 4 | 3 | 3 | 3 |
| `blocked_shot` | 10 | 10 | 10 | 0 |
| `ball_recovery` | 3 | 3 | 3 | 2 |
| `dribbled_past` | -8 | -6 | -6 | 0 |
| `foul` | -3 | -3 | -3 | -3 |
| `foul_drawn` | 3 | 3 | 3 | 3 |
| `penalty_won` | 30 | 30 | 30 | 0 |
| `penalty_committed` | -60 | -60 | -60 | -60 |
| `error_to_shot` | -40 | -40 | -40 | -50 |
| `error_to_goal` | -120 | -120 | -120 | -150 |
| `save` |  |  |  | 15 |
| `save_inside_box` |  |  |  | 5 |
| `punch` |  |  |  | 2 |

Known limits of v1: the volume terms (accurate passes) push midfielders up, and on a sample of eight Serie A matches
the median was 6.34 with about a quarter of players above 7, well above the provider's spread. The weights have not been tuned yet.
A calibration report that compares the sources and suggests a `curve` is planned; it will never be applied automatically.

## Changing the parameters

The parameters are in [`internal/voto/openstat/params_v1.json`](https://github.com/cortomuso/openfanta/blob/main/internal/voto/openstat/params_v1.json), embedded in the binary.

1. Copy the file to `params_v2.json`, change the numbers and set `"version": "v2"`. Unknown terms, positions and keys are rejected when loading.
2. Point `paramsV1` in `internal/voto/openstat/params.go` at the new file and update the tests that name the version.
3. Run `openfanta recompute`. The source's version is part of each run's input hash, so every fixture gets a new run for `openfanta-stat` and `average`, and the old runs stay queryable at `GET /v1/fixtures/{id}/runs`.

Existing data needs `openfanta recompute --reparse` once before `openfanta-stat` has anything to work with, because the stats are stored in the facts from parser version 4.
