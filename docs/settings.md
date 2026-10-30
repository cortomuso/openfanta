---
title: "League settings"
description: "The settings document that says how a league scores, and how its history is kept."
---

A league's scoring is one JSON document. The code is the reference, and this page says where to look.

- **Schema:** [`internal/settings/schema/v1.json`](https://github.com/cortomuso/openfanta/blob/main/internal/settings/schema/v1.json) (JSON Schema 2020-12). Unknown keys are rejected everywhere except `meta` and `x-*`.
- **Classic default:** [`internal/settings/classic.json`](https://github.com/cortomuso/openfanta/blob/main/internal/settings/classic.json). Ingest scores under it until leagues score under their own settings. `GET /v1/settings/default` and `openfanta settings` print it.
- **Validation:** `settings.Parse` in [`internal/settings/settings.go`](https://github.com/cortomuso/openfanta/blob/main/internal/settings/settings.go) canonicalizes, validates against the schema, then checks the engine, `requires` and the event rules a schema cannot express.
- **Canonical form and hash:** `settings.Canonicalize` in [`internal/settings/canonical.go`](https://github.com/cortomuso/openfanta/blob/main/internal/settings/canonical.go) produces RFC 8785 JSON. The sha256 of those bytes identifies the document, and the canonical bytes are what gets stored.
- **Custom rules:** the `team` section holds `expr.cel/1` modules (schema 1.1 or later, listed in `requires`). `CompileExpr` in [`internal/settings/expr.go`](https://github.com/cortomuso/openfanta/blob/main/internal/settings/expr.go) enforces the limits; a runtime failure is a deterministic `RuleError`, not a crash. The golden results in `internal/settings/testdata/cel/` must be rerun on every cel-go upgrade.
- **Change log:** `settings.Diff` in [`internal/settings/diff.go`](https://github.com/cortomuso/openfanta/blob/main/internal/settings/diff.go).
- **History:** `Store.AppendSettings`, `SettingsHistory`, `SettingsDiff` and `VerifyChain` in [`internal/store/settings.go`](https://github.com/cortomuso/openfanta/blob/main/internal/store/settings.go).

All point values are integer hundredths: `300` means +3 and `-50` means -0.5. Floats are rejected.

## How the player rules are applied

For each player who appeared in a match:

```
fantavoto = voto + Σ (count × value)   for every event with a value
```

The result carries a breakdown with one entry per event, so every number can be audited. The mapping from document keys to breakdown codes is `value` in [`internal/engine/engine.go`](https://github.com/cortomuso/openfanta/blob/main/internal/engine/engine.go).
Today the engine understands the per-event values, `player.cards.red_replaces_yellow`,
`clean_sheet.min_minutes` and `player.senza_voto`. It does not tell defenders, midfielders and
attackers apart yet, so only a `P` (goalkeeper) entry under `by_role` has an effect.

Assists are scored by class: `assist.open_play` (breakdown code `assist`), `assist.set_piece` (`assist_set_piece`) and `assist.contribution` (`assist_contribution`). A class with no value in the document scores nothing. The provider decides which class an assist belongs to. Sportmonks does not say: the `sub_type_id` of a goal is how it was struck (left foot, right foot, header), and no structured field marks a set piece or a contribution. So every Sportmonks assist is open play, and the other two classes stay unused until another source can supply them. The facts model already carries a class on each goal event for that.

Two more events exist: `decisive_goal.win` and `decisive_goal.draw` (breakdown codes `decisive_goal_win`, `decisive_goal_draw`), and `player_of_the_match`. The engine counts the score from the regulation goal events (recupero counts; tempi supplementari and shoot-outs never do). The winning goal is the winner's goal number "loser's goals + 1", the one that put them ahead for good. The draw goal is the last goal of a drawn match with goals, the one that levelled it. Only goals and penalty goals are credited, so a decisive own goal credits nobody. The player of the match is the man of the match named by the provider. The Sportmonks feed names one for only some fixtures, so a league that scores it will see it for those fixtures only. None of the three has a value in the Classic default.

The clean-sheet bonus goes to a goalkeeper who played, whose team conceded nothing while they were on the pitch, who has a real voto (not a voto d'ufficio), and who played at least `clean_sheet.min_minutes`.

## History

Every league keeps an append-only list of settings versions, each linked to the one before it by hash.

- A version must name an author and a reason, and the document must be different from the current one.
- A version cannot take effect from a matchday that is closed, or from any earlier one. Rule changes are never retroactive.
- `VerifyChain` recomputes every hash and link, so rewritten history is detected even if the database trigger that blocks `UPDATE` and `DELETE` was bypassed.

There is no API or CLI to submit a version yet, and nothing closes a matchday automatically
(`matchdays.closed_at` is only set by `Store.CloseMatchday`). Both come with the league features in [the design note](design.md#8-leagues-design-not-built-yet).
