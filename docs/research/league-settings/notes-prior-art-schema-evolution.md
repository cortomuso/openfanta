---
title: "Notes: prior art and schema evolution"
description: "Research notes on how other platforms model and evolve settings."
---

## Q1. Prior art: how fantasy platforms model scoring/roster/league settings and mid-season changes

### Takeaway
Every major platform uses a flat "stat identifier -> points" map (sometimes with per-position overrides and threshold bonuses) next to a list of roster slots. Rule changes are typically recomputed **retroactively** over the whole season and logged, not pinned per week. ESPN is the clearest documented example: it recalculates all earlier results and writes a Transaction Log entry. Public docs on exact shapes are thin. Most API detail lives in community wrappers.

### Cited Findings
**Sleeper**
- The league object has `scoring_settings` (object), `roster_positions` (array) and `settings` (object), plus `season`, `status` (pre_draft/drafting/in_season/complete) and `previous_league_id`, which links to the prior season's league. The official docs show these as placeholders and don't document their keys. — [Sleeper API docs](https://docs.sleeper.com/)
- Matchup `points` is the "total points for team based on league settings". `custom_points` holds a value "if commissioner overrides points manually". Starters are an ordered list of player IDs, and the bench is derived as players minus starters. — [Sleeper API docs](https://docs.sleeper.com/)
- Sleeper's default PPR values: reception = 1, passing = 0.04/yd, rushing/receiving = 0.10/yd. — [Sleeper support](https://support.sleeper.com/en/articles/6262990-sleeper-player-picks-stats-explained)
- The ffscrapr `ff_scoring()` normalizes Sleeper scoring into a flat table with one row per position per rule, which suggests the native map can be pivoted to (position, stat, points). — [ffscrapr ff_scoring](https://rdrr.io/github/dynastyprocess/ffscrapr/man/ff_scoring.html)

**ESPN (unofficial API)**
- League scoring is read from `scoringSettings.scoringItems` in the `mSettings` view. Third-party tools rely on it to support PPR, half-PPR and custom scoring generically. — [espn-mcp (Glama)](https://glama.ai/mcp/servers/HamCops/espn-mcp); views list in the [ffscrapr ESPN vignette](https://erised.las.iastate.edu/CRAN/web/packages/ffscrapr/vignettes/espn_getendpoint.Rmd)
- Calling several views together can return different results than calling them separately, which shows the unofficial API is quirky and unstable. — [ffscrapr ESPN vignette](https://erised.las.iastate.edu/CRAN/web/packages/ffscrapr/vignettes/espn_getendpoint.Rmd)
- Scoring "can be changed by the League Manager ... at any time during the season". If it changes after the season begins, "the system will automatically change all previous game results to match the changed settings", and "the action will be listed in your league's Transaction Log page". — [ESPN Support: Scoring Settings](https://support.espn.com/hc/en-us/articles/360000069512-Scoring-Settings)
- ESPN help articles contradict each other on whether position/game-start limit changes affect earlier scoring: one says they "will not affect previous scoring", another says they "will affect previous scoring accordingly". Bench/IR slots can be increased mid-season but not decreased. — [ESPN Lock Time A](https://support.espn.com/hc/en-us/articles/360000991771-League-Settings-Lock-Time); [ESPN Lock Time B](https://support.espn.com/hc/en-us/articles/360000069952-League-Setting-Lock-Time); [ESPN Lock Time C](https://support.espn.com/hc/en-us/articles/360000088211-League-Setting-Lock-Time-)
- Official stat corrections flow in automatically up to a deadline. Manual LM corrections can double-credit a team if the system later applies the same fix. This is a hazard of mixing manual overrides with automatic recomputation. — [ESPN Stat Corrections](https://support.espn.com/hc/en-us/articles/360000069552-Stat-Corrections)

**Yahoo Fantasy Sports API**
- League `settings` include `scoring_type` (e.g. "head"), `roster_positions` (array of slots), `stat_categories` (object), `stat_modifiers` ("point values ... for each statistical category"), `uses_playoff`, `playoff_start_week` and `num_playoff_teams`. Values are stringly typed (e.g. "0"/"1"). — [community doc of fantasy/v2/league/{key}/settings](https://mintlify.wiki/AlexIbby/fantasyBasketball/api/league-settings)
- Yahoo describes roster positions, scoring stats, "scoring modifiers" and game style as configurable per league. A game-level `stat_categories` resource lists every available stat for a sport. — [Yahoo Fantasy API docs (community mirror)](https://yahoofantasysportsapidocs.readthedocs.io/guide/); [Game resource](https://yahoofantasysportsapidocs.readthedocs.io/guide/game-resource/)
- In August 2026 Yahoo added advanced custom scoring: per-stat and per-position points, yardage bonuses, distance-tiered FG scoring (e.g. 60+ yd), team-win points, and separate scoring for INT- and fumble-return TDs. — [Fantasy Nerds, 2026-08-26](https://fantasynerds.com/news/story/2026/08/26/yahoo-fantasy-football-unveils-advanced-custom-scoring-options-for-commissioners-1605916)
- Yahoo help pages list per-category points (e.g. basketball BLK = 3, ST = 3, TO = -1). Head-to-head categories vs points are separate league formats. — [Yahoo help SLN6919](https://help.yahoo.com/kb/SLN6919.html); [Yahoo scoring overview](https://help.yahoo.com/kb/SLN6179.html)
- The official developer site now redirects to an overview page. It requires OAuth 2.0 and attribution, and Yahoo may throttle usage. — [Yahoo developer](https://sports.yahoo.com/developer)

**MyFantasyLeague**
- The `TYPE=rules` export returns a league's scoring rules using rule abbreviations. The `allRules` export is the catalog of every supported rule: player/team/coach applicability, abbreviation, short and detailed description. This splits a global rule vocabulary from the league's selection of rules. — [MFL API info](https://api.myfantasyleague.com/2025/api_info?STATE=details&TYPE=rules)

**Fantasy Premier League**
- `bootstrap-static` exposes `game_settings` (rules/config such as budget and squad sizes), `element_types` (positions with squad selection rules) and `element_stats` (stat metadata). Per-gameweek live data includes an `explain` breakdown of each player's points. FPL is a single global ruleset, not per-league. — [FPL API community docs](https://fpl-api-docs.notion.site/Fantasy-Premier-League-API-Documentation-1be2fd947c77804d8e90dd3f22f2ea68); [FantasyTool](https://github.com/niciac/FantasyTool)

**Community norms**
- Commissioner guides advise against mid-season rule changes ("do not implement it until after the playoffs") but say scoring changes don't always need a consensus. — [Footballguys Commissioner Guide VI](https://www.footballguys.com/article/2017commish06)
- A pick'em platform lets weighted scoring be edited for past weeks, which "triggers a re-grade and user emails". This is a precedent for re-grading plus notifying members. — [Pick'em help center](https://intercom.help/entertainment-only-help-center/en/articles/15925897-nfl-pick-em-private-contests-commissioner-setup-scoring)

### Inferences
- The common pattern is (a) a global stat catalog (Yahoo `stat_categories`, MFL `allRules`, FPL `element_stats`), (b) a per-league selection of points for each stat, with optional position overrides and bonuses, and (c) a roster-slot list. A fantacalcio schema maps onto this cleanly: a stat catalog (goal, assist, yellow/red card, penalty saved/missed, own goal, clean sheet, goal conceded for GK) -> per-role points; bonus/malus thresholds; modifiers (difesa, centrocampo); and goal-threshold conversion (soglie gol: 66/72/...).
- ESPN's model (retroactive recalculation plus a log entry) matches the stated requirement ("retroactive allowed but logged"). Prior art does not keep superseded results, so an open backend could improve on it by keeping both the old and new computed results.
- Sleeper's `custom_points` shows that a manual override must be stored separately from computed points so it survives recomputation. ESPN's double-credit warning shows what happens when overrides and automatic corrections interact.
- Stringly typed values (Yahoo "0"/"1") and undocumented maps (Sleeper) are anti-patterns for a third-party-client API. Prefer typed values backed by a published JSON Schema.

### Gaps
- Yahoo's exact `stat_modifiers`/`bonuses` XML nesting couldn't be fetched (the official guide redirects). From memory, and **unverified**: `stat_modifiers > stats > stat { stat_id, value, bonuses > bonus { target, points } }`. Verify against a live response.
- Sleeper's `scoring_settings` key names (e.g. `pass_yd`, `rec`, `bonus_rec_yd_100`) are **unverified**. The docs don't list them. Sleeper's behavior when scoring changes mid-season was not found.
- ESPN's `scoringItems` item fields (statId, points, `pointsOverrides` per slot, `isReverseItem`) are **unverified**. The ffscrapr vignette host refused connection on one mirror.
- No usable sources were found for Fantrax or CBS settings shapes or recalculation policy (no calls budgeted).

## Q2. Schema evolution strategies

### Takeaway
The proven playbook is: additive-only changes with defaults; never reuse or retype an identifier (reserve removed names); preserve unknown fields; carry an explicit version marker; for breaking changes, convert through a central internal model ("hub", Kubernetes) or a chain of version-change modules (Stripe) and upcasters (event sourcing). Pin each consumer to a version so its observed semantics never shift silently.

### Cited Findings
**Protocol Buffers**
- Adding fields is wire-safe: old code keeps the new field as an unknown field. Changing a field number amounts to delete-and-add. Numbers must never be reused, and deleted numbers and names should be marked `reserved`. — [protobuf.dev proto3 guide](https://protobuf.dev/programming-guides/proto3/)
- Proto3 preserves unknown fields when parsing and re-serializing. They are **lost** on JSON serialization or field-by-field copying. — [protobuf.dev](https://protobuf.dev/programming-guides/proto3/)
- Adding enum values is wire-safe but can break exhaustive switches. Unrecognized enum values are preserved. Some type changes (int32<->int64 etc.) are wire-compatible but lossy and "safe only if you control the rollout". — [protobuf.dev](https://protobuf.dev/programming-guides/proto3/)

**Avro / Schema Registry**
- BACKWARD compatibility means a consumer on schema X can read data from X and X-1. BACKWARD_TRANSITIVE extends this to all earlier versions, and BACKWARD is the default. A new field is backward compatible only if it has a default, which the reader fills in for old data. — [Confluent Schema Registry docs](https://docs.confluent.io/platform/current/schema-registry/fundamentals/schema-evolution.md); [Confluent 7.0](https://docs.confluent.io/platform/7.0/schema-registry/avro.html)
- Compatibility follows the Avro spec's schema-resolution rules. — [Confluent 5.4 Avro](https://docs.confluent.io/5.4.11/schema-registry/avro.html)

**JSON Schema**
- By default "any additional properties are allowed" (open model). `additionalProperties: false` closes it, but it only sees properties in the same subschema, so it breaks `allOf` extension. `unevaluatedProperties` (2019-09+) sees properties from subschemas. — [json-schema.org object reference](https://json-schema.org/understanding-json-schema/reference/object)

**CloudEvents**
- A required `specversion` (e.g. "1.0") carries only major.minor. `dataschema` is a URI, and "Incompatible changes to the schema SHOULD be reflected by a different URI". Consumers may ignore unknown extension attributes, and intermediaries should forward them. — [CloudEvents spec](https://github.com/cloudevents/spec/blob/main/cloudevents/spec.md)

**Stripe**
- Versions are named by date. Each account is pinned to the newest version at its first request, and a single request can override this with the `Stripe-Version` header. — [Stripe blog: API versioning](https://stripe.com/blog/api-versioning)
- Each breaking change is a "version change module" (docs + transformation + affected resources). Responses are built in the current format and transformed backward module by module to the target version. Changes with side effects are flagged `has_side_effects` and avoided. The changelog is generated from the modules. — [Stripe blog](https://stripe.com/blog/api-versioning)
- Adding endpoints or fields is safe. Removing, renaming or retyping fields is breaking. — [Stripe blog](https://stripe.com/blog/api-versioning)

**Kubernetes**
- Conversions must round-trip "with no loss of information". All versions convert through an internal "hub" type. New fields need defaults that are semantically equivalent across versions. A compatible change does not alter existing semantics, and anything that succeeded before must still succeed. — [K8s api_changes.md](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api_changes.md)
- Maturity runs alpha -> beta -> stable, and stable allows only strictly compatible changes. Don't make a new version both the preferred and the storage version in the same release, because rollback would leave stored objects undecodable. — [K8s api_changes.md](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api_changes.md)

**Event sourcing upcasters**
- Axon treats the event store as append-only and read-only. Upcasters transform stored events to the current structure at read time, driven by the version stored with each event. The Axon 5.x docs say most scenarios no longer need upcasters, and reserve them for splitting, merging or retyping events. — [Axon 4.12 Event Versioning](https://docs.axoniq.io/axon-framework-reference/4.12/events/event-versioning); [Axon 5.0](https://docs.axoniq.io/axon-framework-reference/5.0/events/event-versioning/)
- Greg Young's *Versioning in an Event Sourced System* covers immutability ("Why can't I update an event?"), type-based versioning with double write, weak schema (mapping/wrapper), negotiation, and copy-replace. Only the table of contents was accessible. — [Leanpub esversioning](https://leanpub.com/esversioning/read)

### Inferences
- For client-submitted settings: put `schema_version` (or a `$schema` URI, CloudEvents style) in every settings document. Accept only additive minor changes within a major version. Keep unknown fields verbatim in stored JSON, since round-tripping through typed structs loses them (the protobuf JSON caveat applies). Reject or flag unknown fields that would *affect scoring*, so an old engine can't silently compute wrong results. A capability or `required_features` list lets the server refuse settings it can't honor.
- Stripe-style modules plus Kubernetes-style hub conversion map onto "upcast stored settings vN -> internal model" at read time, with the original bytes kept immutable.
- Settings keys should be stable string identifiers from a global stat catalog (like MFL `allRules`). Never repurpose a retired key; reserve it, as with protobuf `reserved`.

### Gaps
- The Avro spec's FORWARD/FULL rules weren't fetched directly. Only Confluent's BACKWARD material was confirmed.
- Marten's upcasting and Fowler's own versioning articles weren't fetched.

## Q3. Reproducibility: pinning engine semantics, storing inputs, recomputation

### Takeaway
Store each computed result with references to the exact inputs: settings version (content hash), stat-feed snapshot and engine/semantics version. Pin an engine semantics version per settings version, Stripe-style, so code upgrades never silently change historical numbers. Retroactive rule changes then become new settings versions that trigger explicit, logged recomputation.

### Cited Findings
- Event sourcing lets you "discard the application state completely and rebuild it" from the log, and determine state "at any point in time". — [Fowler, Event Sourcing](https://martinfowler.com/eaaDev/EventSourcing.html)
- Responses from external queries may need to be recorded and replayed so replays are deterministic. Bug fixes can be applied by reprocessing events. — [Fowler](https://martinfowler.com/eaaDev/EventSourcing.html)
- Logic that changes over time belongs in strategy objects tied to a temporal property. Fowler says bi-temporal reprocessing of old events under old rules is complex and should be avoided unless needed. — [Fowler](https://martinfowler.com/eaaDev/EventSourcing.html)
- Stripe pins each account to a version so behavior doesn't change under it. — [Stripe blog](https://stripe.com/blog/api-versioning)
- ESPN recalculates all previous results when scoring changes, and keeps only the log entry. — [ESPN Scoring Settings](https://support.espn.com/hc/en-us/articles/360000069512-Scoring-Settings)
- Stat corrections arrive after the fact (ESPN deadline: the Saturday after the game week). Inputs therefore change too, not just rules. — [ESPN Stat Corrections](https://support.espn.com/hc/en-us/articles/360000069552-Stat-Corrections)

### Inferences
- Suggested model: `settings_versions(league_id, seq, canonical_json, sha256, schema_version, engine_semantics_version, effective_from_matchday, created_by, reason)` plus `results(league_id, matchday, settings_sha256, inputs_sha256 [votes/stat snapshot], engine_version, output, computed_at)`. The result key then identifies its exact inputs, and recomputation produces a *new* result row rather than overwriting.
- Canonicalize JSON (e.g. RFC 8785 JCS; not fetched here) before hashing so equal settings hash equally.
- Two kinds of retroactive change exist: rule changes and source-data corrections (fantacalcio vote revisions, "6 politico"). Both should flow through the same recompute-and-log path.

### Gaps
- No fantasy platform was found documenting per-result input hashing or engine pinning. This is a design inference from general patterns.

## Q4. Audit and transparency for configuration changes

### Takeaway
An append-only, member-visible change log with structured diffs (ESPN's Transaction Log is the baseline) covers the real need. A simple per-league hash chain over the settings versions adds cheap tamper evidence. Full Merkle transparency logs (CT/RFC 9162) with inclusion and consistency proofs are overkill unless untrusted third parties host the server.

### Cited Findings
- ESPN lists every settings change in the league's Transaction Log. — [ESPN Scoring Settings](https://support.espn.com/hc/en-us/articles/360000069512-Scoring-Settings)
- Axon treats the event store as read/append-only. — [Axon Event Versioning](https://docs.axoniq.io/axon-framework-reference/4.12/events/event-versioning)
- Greg Young's chapter on immutable events lists Audit, "Worms" (write-once media) and "Crime" as reasons never to update events. — [Leanpub esversioning ToC](https://leanpub.com/esversioning/read)
- Certificate Transparency v2: "A log is a single, append-only Merkle Tree". Inclusion proofs show an entry is present. Consistency proofs "prove the append-only property". Signed tree heads (size, root hash, timestamp, signature) are published periodically. — [RFC 9162](https://www.rfc-editor.org/rfc/rfc9162.html)
- A re-grade that sends user emails is a precedent for notifying members of retroactive changes. — [Pick'em help](https://intercom.help/entertainment-only-help-center/en/articles/15925897-nfl-pick-em-private-contests-commissioner-setup-scoring)

### Inferences
- Recommended: each settings version stores `prev_sha256`, forming a hash chain. The API exposes the history, JSON diffs (e.g. JSON Patch, RFC 6902, not fetched) and the list of affected results with old and new totals. Members or clients can cache the latest head hash and detect rewrites.
- A Merkle log is useful mainly with many independent leagues on a shared server whose operator isn't trusted. For a self-hostable open-source backend it is likely overkill. A hash chain plus optional signed exports is proportionate.

### Gaps
- No fantasy platform was found offering tamper-evident logs. No benchmarks of hash-chain overhead were sought.
