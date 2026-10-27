# Compose fantacalcio rules from typed, versioned modules

> **Status (2026-10-27):** research input, not the decided design. The project owner later decided
> that **settings changes are never retroactive**: they apply only from an open matchday, and closed
> matchdays change only through manual owner/admin corrections, which are logged and visible to
> every member. All inputs are frozen when a matchday closes. Where this report assumes retroactive
> rule changes, those decisions take precedence. The decided design is in
> [docs/design.md](../../design.md) §8–9. The supporting notes are in this folder.

OpenFanta should store each league's rules as a **typed, declarative settings document**. The document is built from a fixed set of versioned rule modules, one per stage of the scoring pipeline, and it can include a narrow **CEL expression escape hatch** that only returns integers. Every version of the document is append-only, canonically hashed, tied to the previous version by a hash chain, and pinned to an engine-semantics version, so the server can recompute any result exactly. Real fantacalcio rules vary widely, but in a structured way. FCO alone advertises 150+ configurable fields ([FCO](https://www.fantacalcio-online.com/it/regole/differenze-regolamento-magic-gazzetta)). Most of that variety is the same few shapes repeated with different parameters: per-event bonus/malus with per-role overrides, band tables over averages or sums, threshold lists for goal conversion, and ordered tie-breakers. Only a small set of rules reaches beyond the single player. Some need the line-up (substitutions, Mantra's out-of-position malus, defence and captain modifiers). Some need the opponent (midfield modifier, opponent-targeted defence variants, draw and superiority corrections). Some need the calendar (fattore campo), and some need the whole league (Formula 1 points, standings, tie-breakers). So the right design is a staged pipeline with a module catalog at each stage, not a general-purpose language. Prior art backs this up. Yahoo, MFL, FPL, ESPN and Sleeper all separate a global stat catalog from per-league point values, and ESPN recomputes past results when rules change and logs the change ([ESPN](https://support.espn.com/hc/en-us/articles/360000069512-Scoring-Settings)). OpenFanta can go further by keeping both the superseded and the recomputed results. The main costs of this design are three. The project has to maintain a module catalog. It has to keep CEL's environment frozen per version. And it has to accept that genuinely new rule shapes need an engine release unless they fit in an expression.

## Five dependency scopes cover every rule found

The task framed rules as depending on the player, the line-up, the opponent or the league. The evidence needs two refinements. First, "player-only" rules often need the **real match context** as well: gol decisivo needs the match result and goal timeline, and postponement policies apply per real fixture to whole rosters ([Fantacalcio.it](https://www.fantacalcio.it/regolamenti/19_06_2020/regolamento-ufficiale-fantacalcio-412003)). Second, fattore campo depends on the **fixture calendar**, not on the opponent's performance ([FCO](https://www.fantacalcio-online.com/it/regole/come-funziona-regola-del-59-bonus-pareggio/amp)). The table below classifies the observed rules. "Player+M" means the player's own events plus real-match context.

| Rule family | Real-world variety | Scope | Pipeline stage |
|---|---|---|---|
| Base voto source | Fantacalcio journalistic, Voto Statistico FG, Redazione Italia; chosen separately from role and bonus sources ([Fantacalcio.it](https://www.fantacalcio.it/regolamenti/leghe-private)). FCO uses a weighted average of up to 3 sources, rounded to 2 decimals ([FCO](https://www.fantacalcio-online.com/it/regole/regolamento-fantacalcio-guida-completa)) | Player | 1. Fantavoto |
| Core bonus/malus | Goal +3, assist +1, yellow −0.5, red −1, own goal −2, GK conceded −1, penalty saved +3, missed −3 ([Fantacalcio.it](https://www.fantacalcio.it/regolamenti/leghe-private)). FCO uses penalty scored/saved/missed ±2.5 ([FCO](https://www.fantacalcio-online.com/it/regole/regolamento-fantacalcio-guida-completa)). Old Gazzetta rule: GK own goal −1 ([SOS Fanta](https://www.sosfanta.com/news/regolamento-cosa-cambia-tra-assist-malus-per-autogol-e-regola-dei-15-minuti/)) | Player (+role) | 1 |
| Per-role values | Classic per P/D/C/A; Mantra only GK vs outfield ([Fantacalcio.it](https://www.fantacalcio.it/regolamenti/leghe-private)). FCO: any event can differ by role ([FCO](https://www.fantacalcio-online.com/it/regole/regolamento-fantacalcio-guida-completa)) | Player (+role) | 1 |
| Assist variants | In movimento +1 / da fermo +0.5; Soft/Standard/Gold; "contributo al gol", off by default ([Fantacalcio.it](https://www.fantacalcio.it/regolamenti/leghe-private), [SOS Fanta](https://www.sosfanta.com/news/rivoluzione-assist-fantacalcio-it-cosa-cambia-leghe-contributo-al-gol/)). Key passes; FCO removed some subtypes in Aug 2026 ([FCO](https://www.fantacalcio-online.com/it/regole/bonus-malus-avanzati-fantacalcio-come-funzionano)) | Player | 1 |
| Advanced events | Woodwork +1 (30%), penalty won +0.5 (29%), penalty conceded −0.5 (28%), decisive error −0.5 (27%), clean sheet 90' +1 (42%) ([FCO](https://www.fantacalcio-online.com/it/regole/bonus-malus-avanzati-fantacalcio-come-funzionano)) | Player | 1 |
| Card cap | Maximum card malus −1, no "−1.5". A card counts off-pitch only if the player entered the match ([Fantacalcio.it](https://www.fantacalcio.it/regolamenti/19_06_2020/regolamento-ufficiale-fantacalcio-412003)) | Player | 1 |
| Gol decisivo | +1 for a goal that decides a win and/or a draw, optional ([Fantacalcio.it](https://www.fantacalcio.it/regolamenti/leghe-private)) | Player+M | 1 |
| Senza voto | No minimum minutes on Fantacalcio.it. s.v. + red = 4. s.v. + any other event = 6 ± event. s.v. + yellow = substitute or fixed 5.5 ([Fantacalcio.it](https://www.fantacalcio.it/regolamenti/19_06_2020/regolamento-ufficiale-fantacalcio-412003)). Gazzetta 15'/25' thresholds ([SOS Fanta](https://www.sosfanta.com/news/regolamento-cosa-cambia-tra-assist-malus-per-autogol-e-regola-dei-15-minuti/)). FCO voto d'ufficio 3–6; ~94% of leagues choose substitution ([FCO](https://www.fantacalcio-online.com/it/voti-oggettivi/come-funziona-voto-oggettivo-minutaggio-senza-voto)) | Player (+minutes) | 1 |
| Postponed / suspended / forfeit | Wait, count if brought forward, or "6 politico" to the whole roster ([Fantacalcio.it](https://www.fantacalcio.it/regolamenti/leghe-private)). FCO allows 5/5.5/6 or 6 outfield / 5 GK ([FCO](https://www.fantacalcio-online.com/it/guide/gestione-leghe/gestione-partite-rinviate)). Suspended matches have their own options | Player+M (per fixture) | 0. Inputs |
| Substitutions | Classic: Dinamic/Hybrid/Traditional; ≥5 subs recommended ([Fantacalcio.it](https://www.fantacalcio.it/regolamenti/leghe-private)). FCO: change of formation in 60% of leagues ([FCO](https://www.fantacalcio-online.com/it/regole/come-funzionano-riserve-dufficio)). Mantra: block subs in bench order, BASIC/EASY/MASTER ([Fantacalcio.it](https://www.fantacalcio.it/regolamenti/sistema-mantra)) | Line-up | 2. Line-up resolution |
| Riserva d'ufficio | Fixed vote (e.g. 4, GK 2), counts as a sub. FCO most common: GK 3, outfield 5 or 4; disabled in ~1/3 of leagues ([FCO](https://www.fantacalcio-online.com/it/regole/come-funzionano-riserve-dufficio)) | Line-up | 2 |
| Mantra out-of-position | −1 per player; some placements forbidden at the start, allowed during subs ([Fantacalcio.it](https://www.fantacalcio.it/regolamenti/sistema-mantra)) | Line-up | 2 |
| Missing line-up | Reuse the last one (favoured), punteggio politico, or forfeit ([Fantacalcio.it](https://www.fantacalcio.it/regolamenti/leghe-private)) | Line-up | 2 |
| Defence modifier | GK + 3 best defenders, voti puri: 6.00/6.50/7.00 → +1/+3/+6, needs 4 defenders. Used on 72.3% of matchdays, avg +1.86. Six-band and opponent-targeted variants exist ([FCO](https://www.fantacalcio-online.com/it/regole/guida-modificatore-difesa/amp), [FCO](https://www.fantacalcio-online.com/it/regole/modificatori-consigli-guida-fantacalcio-esempi)) | Line-up (some: Opponent) | 3. Team |
| Captain | Band table on the captain's voto puro, −2.5 to +2.5; most-used FCO modifier at 11% ([FCO](https://www.fantacalcio-online.com/it/regole/regola-capitano-modificatore-come-funziona/amp)) | Line-up | 3 |
| Attack, GK, fair play, rendimento, modulo, "squadra perfetta" | Mostly band or boolean conditions over the starting XI ([FCO](https://www.fantacalcio-online.com/it/regole/modificatori-consigli-guida-fantacalcio-esempi), [Fantamaster](https://www.fantamaster.it/guide-leghe-fantacalcio-fantamaster/category/bonus-e-malus/)). Mantra D-Factor and R-Factor are mutually exclusive ([Fantacalcio.it](https://www.fantacalcio.it/regolamenti/sistema-mantra)) | Line-up | 3 |
| Midfield modifier | Compares the SUM of each side's midfielders; zero-sum ±0.5 to ±4 in 1-point gap bands ([FCO](https://www.fantacalcio-online.com/it/regole/guida-modificatore-centrocampo)) | Opponent | 4. Match |
| Fattore campo | Fixed home bonus before conversion: +2 recommended ([Fantacalcio.it](https://www.fantacalcio.it/regolamenti/leghe-private)), +1 at Fantamaster, toggled per competition ([Fantamaster](https://www.fantamaster.it/regole-fantacalcio-regolamento-asta-moduli-bonus-malus/)) | Calendar | 4 |
| Fasce gol | 66 = 1 goal in 96% of rulebooks. FCO preset 66/72/77/81/85/89/93/97/101 ([FCO](https://www.fantacalcio-online.com/it/regole/come-funziona-regola-del-59-bonus-pareggio/amp)). Fantamaster +6 per goal ([Fantamaster](https://www.fantamaster.it/regole-fantacalcio-regolamento-asta-moduli-bonus-malus/)) | Team total | 4 |
| Corrections | Bonus pareggio (gap < 3), superiorità (gap ≥ 4, min 59), regola del 59, umiliazione ([FCO](https://www.fantacalcio-online.com/it/regole/come-funziona-regola-del-59-bonus-pareggio/amp)). "Autogol" under 55 ([Fantacalcio.it](https://www.fantacalcio.it/regolamenti/leghe-private)) | Opponent | 4 |
| Competition formats | H2H campionato (~72%), coppa (~20%), tutti contro tutti (~35%), F1 (~5%), gironi + KO. ~1 in 3 leagues run 2+ competitions sharing roster and market ([FCO](https://www.fantacalcio-online.com/it/gestione-leghe/gestione-competizioni/amp)). Battle Royale, Highlander, Old Champions, Tabellone ([Fantacalcio.it](https://www.fantacalcio.it/regolamenti/leghe-private)) | League | 5. Standings |
| Tie-breakers, odd team counts | Total fantapunti, goals scored/conceded, difference, classifica avulsa. Phantom team or rest round ([Fantacalcio.it](https://www.fantacalcio.it/regolamenti/leghe-private)) | League | 5 |
| Roster and market | 25 = 3/8/8/6. Classic minimum 17 (1/6/6/4), maximum 90. Mantra ≥23 with 2 GK. 250/280 credits with official prices, free amount with friends. Buste chiuse, Asta Smart, session caps, svincolo refunds ([Fantacalcio.it](https://www.fantacalcio.it/regolamenti/leghe-private)) | League | Validation, not scoring |

The pattern matters more than any single row. Stage 1 is a **pure function of one player's match record plus a role**, so it parallelizes trivially and could even be shared across leagues using the same sources. Stages 2–3 need only one team's submitted line-up. Only stages 4–5 couple teams together. A settings format organized by stage makes it obvious which results a change touches. Changing a midfield-modifier band recomputes matches. Changing the goal bonus for defenders recomputes every fantavoto downstream.

## Recurring shapes make a module catalog sufficient

Each family above reduces to a small number of parameterized shapes. **Event-value maps** with role overrides cover all stage-1 bonus/malus, assist categories and advanced events. One wrinkle: some events interact. Penalty goals are +3 as normal goals on Fantacalcio.it but +2.5 on FCO ([FCO](https://www.fantacalcio-online.com/it/regole/regolamento-fantacalcio-guida-completa)), so a penalty value must declare whether it *replaces* or *adds to* the goal value. The card cap is another interaction. **Band tables** over an aggregate of selected players cover the defence, captain, attack, GK, D-Factor and rendimento modifiers. They differ only in player selection (GK + best 3 defenders, captain, all forwards meeting conditions), aggregate (average or sum, voto puro or fantavoto), minimum count and voto d'ufficio for gaps, and target (self, opponent, or zero-sum). The notes' own inference matches this generic "selection → aggregate → bands → target" engine ([FCO](https://www.fantacalcio-online.com/it/regole/guida-modificatore-difesa/amp)). **Monotone threshold lists** cover goal conversion. **Ordered predicate rules** on (scoreA, scoreB, goalsA, goalsB) cover the corrections. **Points tables and tie-breaker lists** cover competitions.

Integer hundredths fit every shape, but two determinism traps need explicit rules. First, averages of hundredths are not integers: four voti summing to 2,601 average 650.25. Band membership should therefore be defined as `sum ≥ threshold × n`, never by dividing. Second, published band tables mix half-open ranges ("6.00–6.49") with odd inclusive ones ("6.01–6.25") ([FCO](https://www.fantacalcio-online.com/it/regole/guida-modificatore-difesa/amp)). The schema should express bands only as ascending inclusive lower bounds (`from: 650`), so a 6.495 average has exactly one band. Selection ties need the same care. "Best 3 defenders" with equal votes, or Mantra's combinatorial sub search over bench order ([Fantacalcio.it](https://www.fantacalcio.it/regolamenti/sistema-mantra)), need a specified tie-break such as bench or slot order. Rounding is a real gap. The research found **no platform that documents its fantavoto rounding** beyond FCO's two-decimal weighted average. OpenFanta should make the rounding step and mode explicit fields and not inherit any implicit behavior.

Not everything is a parameter. The three Classic substitution modes and Mantra's BASIC/EASY/MASTER block substitution ([Fantacalcio.it](https://www.fantacalcio.it/regolamenti/sistema-mantra)) are algorithms, not formulas. They belong in engine code, selected by an enum and versioned with the engine. Asking league admins to express Mantra's search order in any expression language would invite bugs and divergence.

## CEL wins as escape hatch, not as the primary format

Making an expression language the primary format is tempting: maximal flexibility, no catalog to maintain. The evidence argues against it. House rules exist that the catalog will not anticipate, such as Fantamaster's 2026/27 "Modificatore Modulo" and "Squadra Perfetta" ([Fantamaster](https://www.fantamaster.it/guide-leghe-fantacalcio-fantamaster/category/bonus-e-malus/)) or graded Mantra maluses ([Goal.com](https://www.goal.com/it/liste/fantacalcio-idee-regole-nuove-lega/bltec3ec477596b4a6e)). But most leagues change parameters, not structure. Declarative values render cleanly as "Gol difensore: +4,50", diff field by field, and validate against a published schema. Expressions do none of that without extra tooling. Production systems converge on the hybrid. Kubernetes puts CEL one-liners into typed fields of declarative resources ([Kubernetes](https://kubernetes.io/docs/reference/using-api/cel/)). Google IAM uses a CEL subset inside bindings ([Google IAM](https://docs.cloud.google.com/iam/docs/conditions-overview)). flagd embeds JSONLogic with custom deterministic operators inside a flag schema ([flagd](https://flagd.dev/reference/flag-definitions/)).

Among the languages, **CEL** has the strongest guarantees. Its spec promises deterministic evaluation to a value or error, guaranteed termination, no side effects, and integer overflow raised as an error rather than wrapped ([cel-spec](https://github.com/google/cel-spec/blob/master/doc/langdef.md)). Its cost units are hardware-independent and deterministic, which Kubernetes uses both to reject expensive expressions at write time and to cap runtime ([Kubernetes](https://kubernetes.io/docs/reference/using-api/cel/)). Third-party clients matter here, because they may want to preview scores locally, and CEL has an official Python implementation since March 2026 ([Google](https://opensource.googleblog.com/2026/03/announcing-cel-expr-python-the-common-expression-language-in-python-now-open-source.html)) plus an ECMAScript evaluator with upstream conformance data ([Buf](https://cdn.jsdelivr.net/npm/@bufbuild/cel-spec@0.4.0/README.md)). The alternatives each fail on a specific requirement:

| Language | Disqualifier for this use |
|---|---|
| Expr | No formal spec, no ports found. CVE-2025-29786 parser OOM, fixed in 1.17.0 ([GitLab](https://advisories.gitlab.com/pkg/golang/github.com/expr-lang/expr/CVE-2025-29786/)) |
| Starlark | Exact big ints and ordered dicts are good, but it is a full language. Termination relies on step limits ([starlark-go](https://raw.githubusercontent.com/google/starlark-go/master/doc/spec.md)), and porting it to other languages is a big job |
| JSONLogic | No formal spec, no number type. Ports disagree on truthiness and equality ([python-jsonlogic](https://pypi.org/project/python-jsonlogic)) |
| Rego | Nondeterministic builtins (`time.now_ns`, `rand.intn`, `http.send`) ([OPA](https://www.openpolicyagent.org/docs/policy-reference/builtins)); Datalog shape suits policy, not arithmetic |
| Lua (gopher-lua) | `float64`-only numbers ([gopher-lua](https://github.com/yuin/gopher-lua)) break bit-exact hundredths |
| Wasm | Deterministic only with care around NaNs and host functions ([WebAssembly](https://www.webassembly.org/docs/nondeterminism/)). Opaque to league members, so it fails auditability |

CEL has one real weakness for long-lived leagues: library churn. cel-go moved to `cel.dev/cel-go` in v0.32.0, and v0.33.0 changed the default cost model, a breaking change ([cel-go releases](https://github.com/cel-expr/cel-go/releases)). The old google/cel-go repository will eventually be removed ([google/cel-go](https://github.com/google/cel-go)). Reproducibility "years later" therefore requires OpenFanta to **pin a named CEL environment version** (declared variables, custom functions, cost limits, library features), as Kubernetes does when it gates CEL features by server version ([Kubernetes](https://kubernetes.io/docs/reference/using-api/cel/)). Results must also be tested against golden fixtures whenever cel-go is upgraded. The environment should declare no `double`-producing functions, and the server should reject any expression whose checked result type is not `int`. Security follows the same layered model the research found. Before parsing, cap source length and AST node count, which cel-go added in v0.30.0. Reject expressions whose estimated cost exceeds a cap at write time. Enforce a runtime cost budget. Expose plain declared data only. Record any runtime error as a deterministic "rule error" outcome so the failure itself is reproducible.

## Versioned documents, pinned semantics and a hash chain make retroactivity safe

Schema evolution has a well-established playbook, and OpenFanta should adopt it whole. Make changes additive within a major version, with defaults that preserve old meaning, as Avro/Confluent require for backward compatibility ([Confluent](https://docs.confluent.io/platform/current/schema-registry/fundamentals/schema-evolution.md)). Never reuse or retype an identifier, and reserve retired names, as protobuf does ([protobuf.dev](https://protobuf.dev/programming-guides/proto3/)). Put an explicit version marker in every document, as CloudEvents' `specversion` does ([CloudEvents](https://github.com/cloudevents/spec/blob/main/cloudevents/spec.md)). For breaking changes, convert stored documents through a single internal "hub" model that round-trips without loss, as Kubernetes does ([Kubernetes](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api_changes.md)), using Stripe-style per-change transformation modules ([Stripe](https://stripe.com/blog/api-versioning)). The original bytes stay immutable, as event-sourcing upcasters keep them ([Axon](https://docs.axoniq.io/axon-framework-reference/4.12/events/event-versioning)).

Unknown fields need a policy specific to scoring. JSON Schema's open model allows unknown properties by default ([json-schema.org](https://json-schema.org/understanding-json-schema/reference/object)). For scoring that is dangerous: an older server that silently ignores a new `penalty_goal.mode` would compute wrong results with full confidence. The rule should be that **unknown keys inside scoring modules are rejected**, while a reserved `x-` namespace and a `meta` block carry client UI data the engine ignores but preserves verbatim. Protobuf's warning that unknown fields are lost on JSON re-serialization or field copying ([protobuf.dev](https://protobuf.dev/programming-guides/proto3/)) means the server must store the submitted canonical JSON, not a re-marshalled Go struct. A `requires` list of feature identifiers lets a server refuse documents it cannot honor, rather than misinterpret them.

Two version axes must stay separate. **`schema`** governs the document shape. **`engine`** governs semantics: what "best 3 defenders" means on ties, how Hybrid substitution resolves, which CEL environment applies. Stripe pins each account to the API version current at its first request ([Stripe](https://stripe.com/blog/api-versioning)). OpenFanta should likewise pin each settings version to an engine-semantics version and keep old semantics executable, so a bug fix never silently rewrites last season. Adopting a fix becomes an explicit, logged settings change (bump `engine`), which flows through the same recompute path as any rule change.

Prior art sets a low bar for auditing. ESPN recomputes all past results and writes only a Transaction Log entry ([ESPN](https://support.espn.com/hc/en-us/articles/360000069512-Scoring-Settings)), and its help pages even contradict each other on whether roster-slot changes affect past scoring ([ESPN A](https://support.espn.com/hc/en-us/articles/360000991771-League-Settings-Lock-Time), [ESPN B](https://support.espn.com/hc/en-us/articles/360000069952-League-Setting-Lock-Time)). OpenFanta should keep **every computed result as an immutable row**, keyed by settings hash, input-snapshot hash and engine version. A recomputation then produces new rows plus a member-visible diff of old and new totals and standings. Inputs change as well as rules: vote revisions, VAR-reverted events ([Fantacalcio.it](https://www.fantacalcio.it/regolamenti/19_06_2020/regolamento-ufficiale-fantacalcio-412003)), 6 politico decisions and stat corrections after the deadline ([ESPN](https://support.espn.com/hc/en-us/articles/360000069552-Stat-Corrections)). Input changes should flow through the same versioned, logged path. Manual admin overrides belong in a separate layer, as Sleeper keeps `custom_points` apart from computed `points` ([Sleeper](https://docs.sleeper.com/)), because ESPN shows how mixing manual fixes with automatic recomputation double-credits teams ([ESPN](https://support.espn.com/hc/en-us/articles/360000069552-Stat-Corrections)). For tamper evidence, a per-league **hash chain** (each version stores the previous version's hash) is proportionate. Clients can cache the head hash and detect rewritten history. Full Merkle transparency logs with consistency proofs ([RFC 9162](https://www.rfc-editor.org/rfc/rfc9162.html)) only pay off if league members do not trust whoever operates the server.

## The recommended design in one document

The sketch below shows a Classic league with one CEL house rule, rendered in YAML for readability. The canonical stored form is JSON, canonicalized (for example with RFC 8785 JCS) before hashing. All scores, votes and thresholds are integer hundredths.

```yaml
schema: "openfanta.settings/1.4"        # document shape: major.minor, additive within major
engine: "2026.10"                       # pinned semantics (tie-breaks, sub algorithms, CEL env)
requires: ["scoring.penalty_mode", "expr.cel/1"]   # server must support all, or reject
meta: { name: "Lega del Bar", x-client: { theme: "verde" } }   # ignored by engine, preserved

game: classic                           # classic | mantra -> selects role catalog and stage-2 algorithms
roles: { catalog: "classic", groups: { GK: [P], OUT: [D, C, A] } }

sources:
  voto:   { inputs: [ { source: "fantacalcio", weight: 1 } ],
            combine: weighted_mean, rounding: { step: 1, mode: half_up } }   # step in hundredths
  events:  "fantacalcio"
  assists: "fantacalcio"
  roles:   "fantacalcio"

player:                                 # STAGE 1 - pure per-player
  events:                               # catalog IDs; value = default, by_role overrides
    goal:              { value: 300, by_role: { D: 450 } }
    penalty_goal:      { mode: same_as_goal }            # or replace_goal / add_to_goal + value
    assist.open_play:  100
    assist.set_piece:  50
    assist.contribution: 0
    yellow:            -50
    red:               -100
    own_goal:          { value: -200 }
    goal_conceded:     { by_role: { P: -100 } }
    penalty_saved:     300
    penalty_missed:    -300
    clean_sheet:       { by_role: { P: 100 }, min_minutes: null }
    decisive_goal.win: 100
    decisive_goal.draw: 0
  caps: { cards_min: -100 }
  senza_voto:
    default: substitute
    with_red: { fixed: 400 }
    with_yellow: substitute                    # or { fixed: 550 }
    with_other_events: { base: 600 }           # 6 + bonus/malus
    min_minutes: null                          # or { outfield: 15, GK: 25 } for Magic-style
  fixture_exceptions:                          # STAGE 0 - per real match
    postponed: { mode: ufficio, vote: { OUT: 600, GK: 600 }, bonus_malus: false, scope: whole_roster }
    suspended: { mode: wait }

lineup:                                 # STAGE 2 - own line-up
  formations: ["3-4-3","3-5-2","4-5-1","4-4-2","4-3-3","5-4-1","5-3-2"]
  bench: { size: 11, ordered: true }
  substitutions: { max: 5, algorithm: "classic.hybrid/1" }
  riserva_ufficio: { enabled: true, vote: { OUT: 400, GK: 200 }, max: 1, counts_as_sub: true }
  missing_lineup: reuse_last
  captain: { mode: optional, vice: true }

team:                                   # STAGE 3 - modifiers on own XI
  - id: mod_difesa
    type: "band/1"
    select: { from: starters, take: [ { role: P, n: 1 }, { role: D, n: 3, best_by: voto_puro } ],
              require: { D: 4 }, tie_break: slot_order }
    aggregate: { of: voto_puro, op: mean }    # evaluated as sum >= from*n, never divided
    bands: [ { from: 600, add: 100 }, { from: 650, add: 300 }, { from: 700, add: 600 } ]
    target: self
  - id: capitano
    type: "band/1"
    select: { from: captain_or_vice }
    aggregate: { of: voto_puro, op: mean }
    bands: [ { from: 0, add: -250 }, { from: 400, add: -200 }, { from: 600, add: 0 }, { from: 850, add: 250 } ]   # abbreviated FCO table
    target: self
  - id: tutti_sufficienti
    type: "expr.cel/1"
    stage: team
    cel: "lineup.starters.all(p, p.voto_puro >= 600) ? 200 : 0"   # must type-check to int
    description: "+2 se tutti gli 11 titolari prendono almeno 6"

match:                                  # STAGE 4 - opponent and calendar
  home_bonus: 200
  pairwise:
    - { id: mod_centrocampo, type: "duel/1", select: { role: C, n: 4, ufficio: [500, 450] },
        aggregate: { of: voto_puro, op: sum },
        bands: [ { gap_from: 100, swing: 50 }, { gap_from: 200, swing: 100 } ], target: zero_sum }
  goals: { thresholds: [6600, 7200, 7700, 8100, 8500], then_every: 400 }
  corrections:
    - { type: "draw_bonus/1", max_gap: 300 }
    - { type: "superiority/1", min_gap: 400, min_score: 5900 }

competitions:                           # STAGE 5 - league
  - { id: campionato, format: "h2h_round_robin/1", legs: 2, points: { win: 3, draw: 1, loss: 0 },
      tiebreakers: [fantapoints_total, goals_for, head_to_head, goal_difference], odd_teams: rest }
  - { id: coppa, format: "knockout/1", overrides: { match.home_bonus: 0 } }

roster:                                 # validation only, never affects scores
  size: 25
  quotas: { P: 3, D: 8, C: 8, A: 6 }
  budget: 500
  market: { type: sealed_bid, session_cap: 5, release_refund_pct: 50 }
```

Around the document, the server keeps append-only `settings_versions(league, seq, canonical_json, sha256, prev_sha256, schema, engine, effective_from_matchday, author, reason, created_at)` rows. A submission arrives as a full document plus an optional client patch, and the server stores the full canonical document plus a computed structured diff. When `effective_from_matchday` lies in the past, the server enqueues recomputation of the affected matchdays from the earliest stage the diff touches. It writes new result rows keyed `(matchday, settings_sha256, inputs_sha256, engine)` and publishes a "recompute report" listing every changed fantavoto, match result and standings position. The engine also emits a per-player and per-team **explain trace**: which module contributed how many hundredths. FPL's per-player `explain` breakdown is a precedent ([FPL docs](https://fpl-api-docs.notion.site/Fantasy-Premier-League-API-Documentation-1be2fd947c77804d8e90dd3f22f2ea68)). The trace is how non-technical members audit a CEL rule they cannot read. Mantra drops into the same skeleton: `game: mantra`, the Mantra role catalog, roles grouped GK/outfield, `substitutions.algorithm: "mantra.basic/1"`, an `out_of_position_malus: -100` field at stage 2, and a D-Factor `band/1` module. A validation rule forbids D-Factor and R-Factor together ([Fantacalcio.it](https://www.fantacalcio.it/regolamenti/sistema-mantra)).

The trade-offs are concrete. The catalog approach makes the common 95% of leagues renderable, diffable and portable, but it **couples new rule shapes to engine releases**: anything a `band`, `duel` or `expr` module cannot express waits for a new module type. CEL closes most of that gap. In return OpenFanta takes on a frozen-environment commitment, golden-test discipline across cel-go upgrades, and expressions that are harder for members to read than tables. Keeping old engine versions executable grows maintenance cost with each release. The alternative is Fowler's warning about bi-temporal reprocessing under old rules ([Fowler](https://martinfowler.com/eaaDev/EventSourcing.html)): upgrade engines only by explicit league action, and accept that frozen leagues run old code paths. Rejecting unknown scoring keys protects correctness but forces clients to check `requires` and handle refusals gracefully. Several facts remain unverified and should be confirmed before they are encoded as defaults: current Gazzetta Magic rules (the research relied on 2016 secondary sources), official fantavoto rounding behavior, Mantra per-formation slot grids (published only as images), and when the vice-captain replaces the captain.

### Questions the project owner must settle

| Question | Why it matters |
|---|---|
| Does a retroactive change apply from a chosen matchday, or always from matchday 1? Who may make one, admin alone or with member approval? | Decides the recompute scope and the governance model |
| Are stage-1 fantavoti per league, or shared across leagues with identical sources and player settings? | A large performance saving, but the cache key must include the settings hash |
| Are vote-provider corrections pulled automatically until a deadline, or frozen at matchday close? | Input versioning and the ESPN-style double-credit risk |
| Should CEL be enabled from v1, or deferred until the module catalog stabilizes? | Security surface and long-term pinning burden |
| Are the rounding mode and step a league choice, or one global protocol rule? | Determinism across clients that preview scores |
| How long must old engine versions remain executable? | Maintenance cost versus permanent reproducibility |
| Should ratings and events be stored inside the hashed input snapshot, or referenced from the provider? | Whether results stay verifiable if the provider later revises its data |
| Is a signed export or a Merkle log needed for leagues on third-party-hosted instances? | Whether a hash chain alone gives enough tamper evidence |
| Do roster and market rules version together with scoring, or separately? | Market changes should not trigger score recomputation |

## Conclusion

The research changes the framing of the question. It is less "how flexible should the format be" and more "where should flexibility live". Fantacalcio's variety is wide but shallow: a few shapes with many parameters, plus a handful of algorithms (substitutions, Mantra placement) that should never be user-scripted. That points to a design where the **module catalog is OpenFanta's real API surface**, versioned with the same care as protobuf fields. CEL serves as a pressure valve whose usage data shows which modules to add next. Every expression that recurs across leagues is a candidate for promotion into a typed module.

The second insight is that auditability and determinism are the same problem viewed from two sides. Immutable result rows keyed by content hashes, pinned engine semantics, and explain traces make recomputation provable to the engine and legible to members. No mainstream fantasy platform documents doing this. ESPN overwrites and logs. Keeping the history, rather than adding more rule options, is where an open-source source-of-truth backend can most clearly beat the incumbents.
