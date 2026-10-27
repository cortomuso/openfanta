---
title: "Rule sets"
description: "Reference for every key in a rule set file."
---

A rule set is a YAML (or JSON) file that holds every bonus and malus value used to
compute fantavoti. Changing values needs no code change.

- Built-in rule sets live in [`rulesets/`](https://github.com/cortomuso/openfanta/tree/main/rulesets) and are embedded in the binary.
- Files are validated against [`rulesets/schema.json`](https://github.com/cortomuso/openfanta/blob/main/rulesets/schema.json) (JSON Schema 2020-12)
  when they load. Unknown keys are rejected.
- **All point values are integer hundredths**: `300` means +3, `-50` means -0.5. No floats.
- A rule set is identified by `id@version`, e.g. `classic@1.0.0`. A published version is
  **immutable**: every computation run records the rule set it used, along with its content hash.
  To change a value, copy the file, bump `version`, and recompute.

## Formula

For each player who appeared in a match:

```
fantavoto = voto + Σ (count × value)   for every scoring item below
```

The result carries a breakdown with one entry per item, so every number can be audited.

## Keys

### Top level

| Key | Type | Meaning |
|---|---|---|
| `id` | string `[a-z0-9-]` | Rule set family, e.g. `classic` |
| `version` | semver `MAJOR.MINOR.PATCH` | Version within the family |
| `name` | string | Display name |
| `description` | string, optional | Free text |

### `scores` (hundredths, per occurrence)

| Key | Classic | Applies when |
|---|---|---|
| `goal` | 300 | Each goal scored, excluding penalties |
| `penalty_scored` | 300 | Each penalty scored in regulation or extra time (shoot-outs never count) |
| `penalty_missed` | -300 | Each penalty missed or saved, for the taker |
| `penalty_saved` | 300 | Each penalty saved, for the goalkeeper |
| `own_goal` | -200 | Each own goal |
| `assist` | 100 | Each assist |
| `yellow_card` | -50 | Each yellow card (see `red_replaces_yellow`) |
| `red_card` | -100 | Sent off, by a straight red or a second yellow |
| `gk_goal_conceded` | -100 | Goalkeepers only: each goal their team conceded while they were on the pitch, own goals included |
| `gk_clean_sheet` | 100 | Goalkeepers only: no goals conceded while on the pitch (see `gk_clean_sheet_min_minutes`) |

### `options`

| Key | Classic | Meaning |
|---|---|---|
| `red_replaces_yellow` | `true` | A player who was sent off gets no `yellow_card` malus, only `red_card` |
| `gk_clean_sheet_min_minutes` | `0` | Minutes a goalkeeper must play to earn `gk_clean_sheet`. `0` means any appearance |

### `senza_voto` (s.v.)

A player is *senza voto* when the voto source provides no voto, or when they played fewer
than `min_minutes`. An s.v. player gets no voto and no fantavoto, unless a `voto_ufficio`
rule applies.

| Key | Classic | Meaning |
|---|---|---|
| `min_minutes` | `15` | A player with fewer minutes is s.v. even if a voto exists |
| `voto_ufficio` | see below | An ordered list of `{when, voto}`. For an s.v. player, the first rule whose `when` event the player had sets their voto (hundredths). Bonus and malus then apply as usual |

Valid `when` events: `goal`, `penalty_scored`, `penalty_missed`, `penalty_saved`,
`own_goal`, `assist`, `yellow_card`, `red_card`.

Classic: `red_card → 4`, `goal → 6`, `penalty_scored → 6`, `penalty_saved → 6`.

## Adding a rule set

1. Copy `rulesets/classic.yaml` to `rulesets/<id>.yaml`.
2. Set `id`, `version` and `name`, and change the values.
3. Run `make test`. The loader tests reject an invalid file.

A new **kind** of bonus (say, a decisive-goal bonus) needs a new key in the schema and support
in the engine. That is a code change and a schema version bump.
