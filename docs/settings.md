---
title: "League settings"
description: "The settings document that says how a league scores, and how its history is kept."
---

A league's scoring is one JSON document. The code is the reference, and this page says where to look.

- **Schema:** [`internal/settings/schema/v1.json`](https://github.com/cortomuso/openfanta/blob/main/internal/settings/schema/v1.json) (JSON Schema 2020-12). Unknown keys are rejected everywhere except `meta` and `x-*`.
- **Classic default:** [`internal/settings/classic.json`](https://github.com/cortomuso/openfanta/blob/main/internal/settings/classic.json). Ingest scores under it until leagues score under their own settings. `GET /v1/settings/default` and `openfanta settings` print it.
- **Validation:** `settings.Parse` in [`internal/settings/settings.go`](https://github.com/cortomuso/openfanta/blob/main/internal/settings/settings.go) canonicalizes, validates against the schema, then checks the engine, `requires` and the event rules a schema cannot express.
- **Canonical form and hash:** `settings.Canonicalize` in [`internal/settings/canonical.go`](https://github.com/cortomuso/openfanta/blob/main/internal/settings/canonical.go) produces RFC 8785 JSON. The sha256 of those bytes identifies the document, and the canonical bytes are what gets stored.
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
attackers apart yet, so only a `P` (goalkeeper) entry under `by_role` has an effect, and every
assist uses `assist.open_play`.

## History

Every league keeps an append-only list of settings versions, each linked to the one before it by hash.

- A version must name an author and a reason, and the document must be different from the current one.
- A version cannot take effect from a matchday that is closed, or from any earlier one. Rule changes are never retroactive.
- `VerifyChain` recomputes every hash and link, so rewritten history is detected even if the database trigger that blocks `UPDATE` and `DELETE` was bypassed.

There is no API or CLI to submit a version yet, and nothing closes a matchday automatically
(`matchdays.closed_at` is only set by `Store.CloseMatchday`). Both come with the league features in [the design note](design.md#8-leagues-design-not-built-yet).
