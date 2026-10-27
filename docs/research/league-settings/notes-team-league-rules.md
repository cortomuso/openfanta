---
title: "Notes: team and league rules"
description: "Research notes on team-, matchday- and league-level rules."
---

> Dependency legend: [LINEUP] = depends only on your own line-up; [OPP] = depends on the opponent; [LEAGUE] = depends on the whole league or matchday ranking; [CAL] = depends on the fixture calendar.

## Modificatori (difesa, centrocampo, attacco, portiere, fair play, capitano)

### Takeaway
The defence modifier is by far the most common "classic" modificatore: the average of the GK plus the 3 best defenders' voti puri (no bonus or malus), giving +1/+3/+6 at 6.00/6.50/7.00, and it needs at least 4 defenders. The midfield modifier is the main opponent-dependent one. On FCO the most-used modifier is actually the captain (11% of competitions). Every value can be configured on every platform.

### Cited Findings
**Difesa [LINEUP; some variants OPP]**
- Standard (Fantacampionato/Gazzetta "Gaz. 2016") table: average of 4 voti puri (GK + 3 best defenders fielded). Below 6.00: 0. 6.00–6.49: +1. 6.50–6.99: +3. 7.00 and above: +6. It needs 4 defenders on the scoresheet ("a referto"), so the 4th defender must also play. — [FCO guida modificatore difesa](https://www.fantacalcio-online.com/it/regole/guida-modificatore-difesa/amp)
- Fantamaster's rules use the same table (6 → +1, 6.5 → +3, 7 → +6) on the average of the GK and the 3 best defenders, excluding bonus/malus. It can only be used with a formation of at least 4 defenders (4- or 5-man defence). — [Fantamaster regole](https://www.fantamaster.it/regole-fantacalcio-regolamento-asta-moduli-bonus-malus/); [Fantamaster guide bonus e malus](https://www.fantamaster.it/guide-leghe-fantacalcio-fantamaster/category/bonus-e-malus/)
- Six-band variant (GK + 3 best defenders): 6.00 → +1; 6.01–6.25 → +2; 6.26–6.50 → +3; 6.51–6.75 → +4; 6.76–7.00 → +5; 7.01 and above → +6. — [FCO guida difesa](https://www.fantacalcio-online.com/it/regole/guida-modificatore-difesa/amp)
- FCO "classic" variant [OPP]: defenders only, no GK, applied to the opponent's score. Up to 4.99 → opponent +2; 5.00–5.49 → +1; 5.50–5.99 → 0; 6.00–6.49 → −1; 6.50–6.99 → −2; 7.00 and above → −3. Missing defenders get a voto d'ufficio (1st 5, 2nd 4.5). The standard minimum is 4 defenders and the admin can change it. — [FCO guida difesa](https://www.fantacalcio-online.com/it/regole/guida-modificatore-difesa/amp)
- "Esteso/Avanzato" variants: 9 bands from −4 to +4 in quarter-point steps, applied to the opponent. Avanzato has no voto d'ufficio, Esteso has one. — [FCO guida difesa](https://www.fantacalcio-online.com/it/regole/guida-modificatore-difesa/amp); [FCO guida modificatori](https://www.fantacalcio-online.com/it/regole/modificatori-consigli-guida-fantacalcio-esempi)
- FCO usage stats for 2025/26: Gaz. 2016 averages +1.86 points per matchday, gives a bonus on 72.3% of matchdays and never a malus. The "Migliori 3" variant is used by 3.1% of competitions; FCO classic (opponent malus) by 2.5% (−0.23 per matchday on average); Avanzato by 1.6%; Esteso by 0.4%. — [FCO guida modificatori](https://www.fantacalcio-online.com/it/regole/modificatori-consigli-guida-fantacalcio-esempi)
- Fantacalcio.it's private-league guidelines recommend the defence modifier as the only traditional Classic modifier. In Mantra it is replaced by the D-Factor. — [Fantacalcio.it regolamento leghe private](https://www.fantacalcio.it/regolamenti/leghe-private)
- Mantra D-Factor: average of the 5 defensive slots, or 6 with the GK in the "5+1" version. At least 3 of them must be Dc/B/Dd/Ds. It does not trigger if the team falls short of the legal number of players. It cannot be combined with the R-Factor. — [Fantacalcio.it sistema Mantra](https://www.fantacalcio.it/regolamenti/sistema-mantra)

**Centrocampo [OPP]**
- Compares the SUM (not the average) of each side's midfielders' voti puri; the better side gets +X and the other side gets −X. Gap < 1.00 → 0; 1.00–1.99 → ±0.5; 2.00–2.99 → ±1; 3–3.99 → ±1.5; 4–4.99 → ±2; 5–5.99 → ±2.5; 6–6.99 → ±3; 7–7.99 → ±3.5; 8 or more → ±4. Base count is 4 midfielders; missing ones get a voto d'ufficio of 5 (1st) and 4.5 (2nd). — [FCO guida centrocampo](https://www.fantacalcio-online.com/it/regole/guida-modificatore-centrocampo)
- FCO usage: 1.4% of competitions. Active on 65.2% of matchdays, net average ≈ 0 (zero-sum). — [FCO guida modificatori](https://www.fantacalcio-online.com/it/regole/modificatori-consigli-guida-fantacalcio-esempi)
- Fantamaster has the same concept (summed base votes, bonus to one side and malus to the other) with configurable values. — [Fantamaster guide](https://www.fantamaster.it/guide-leghe-fantacalcio-fantamaster/category/bonus-e-malus/)

**Attacco [LINEUP]**
- FCO "Mod. Attacco": based on the forward's voto puro from 6.50 up. Used by 2.9%; zero on 93.2% of matchdays. — [FCO guida modificatori](https://www.fantacalcio-online.com/it/regole/modificatori-consigli-guida-fantacalcio-esempi)
- Fantamaster: only counts forwards who did not score, did not miss a penalty and got a pass mark (all three conditions). Values are set by the admin. — [Fantamaster guide](https://www.fantamaster.it/guide-leghe-fantacalcio-fantamaster/category/bonus-e-malus/)

**Portiere [LINEUP]**
- FCO "Portiere No Goal": +1 if the GK concedes nothing (6.9% usage). "Portiere No Bonus": GK voto puro from 6.50 up (0.2% usage). — [FCO guida modificatori](https://www.fantacalcio-online.com/it/regole/modificatori-consigli-guida-fantacalcio-esempi)
- Fantamaster has no separate GK modifier. Instead it has a "Portiere Imbattuto" bonus, which also applies if the GK goes off injured or is sent off without conceding. — [Fantamaster guide](https://www.fantamaster.it/guide-leghe-fantacalcio-fantamaster/category/bonus-e-malus/)

**Fair play / correttezza, rendimento [LINEUP]**
- Fantamaster "Modificatore Correttezza": applies when none of the 11 scoring players gets a yellow or red card. The value is configurable. — [Fantamaster guide](https://www.fantamaster.it/guide-leghe-fantacalcio-fantamaster/category/bonus-e-malus/)
- Fantacalcio.it Leghe: the "Rendimento" (R-Factor in Mantra: bonus or malus by the number of players with a base vote of at least 6) and "Fair play" modifiers exist in both Classic and Mantra and need 11 players with a vote. — [Fantacalcio.it leghe private](https://www.fantacalcio.it/regolamenti/leghe-private); [Mantra](https://www.fantacalcio.it/regolamenti/sistema-mantra)
- New Fantamaster 2026/27 modifiers: "Modificatore Modulo", based on the FINAL formation after automatic subs, swaps and live subs; "Squadra Perfetta", whose old requirement of all 11 at pass mark was relaxed; and an "MVP bonus". — [Fantamaster guide](https://www.fantamaster.it/guide-leghe-fantacalcio-fantamaster/category/bonus-e-malus/)

**Capitano / vice [LINEUP]**
- FCO preloaded captain table (captain's voto puro): <4 → −2.5; 4–4.49 → −2; 4.5–4.99 → −1.5; 5–5.49 → −1; 5.5–5.99 → −0.5; 6–6.49 → 0; 6.5–6.99 → +0.5; 7–7.49 → +1; 7.5–7.99 → +1.5; 8–8.49 → +2; 8.5 and above → +2.5. The captain does NOT double bonuses. The captain can be mandatory or optional. PRO variants: bonus-only, doubled (−5 to +5), wider neutral band (5.5–6.99 = 0). — [FCO regola capitano](https://www.fantacalcio-online.com/it/regole/regola-capitano-modificatore-come-funziona/amp)
- Fantamaster replaced its old fixed captain bonus/malus with a configurable captain modifier based on the captain's voto puro. — [Fantamaster guide](https://www.fantamaster.it/guide-leghe-fantacalcio-fantamaster/category/bonus-e-malus/)
- Captain is the most-used FCO modifier: 11.0% of competitions, +0.28 points per matchday on average, a malus in 26.6% of the matchdays where it applies. — [FCO guida modificatori](https://www.fantacalcio-online.com/it/regole/modificatori-consigli-guida-fantacalcio-esempi)
- Fantacalcio.it Leghe offers captain plus vice-captain as an option; no numeric values are published in the guideline. — [Fantacalcio.it leghe private](https://www.fantacalcio.it/regolamenti/leghe-private)

### Inferences
- A modifier engine can be generic: (source set of players) → (aggregate: avg or sum, voto puro or fantavoto) → (band table) → (target: self, opponent, or both zero-sum), plus minimum player count, voto d'ufficio for missing players, and an option to include the GK.

### Gaps
- No primary Gazzetta Fantacampionato rule page was found; the "Gaz. 2016" table comes from FCO's description of it.
- When the vice-captain takes over (captain s.v. vs. captain not fielded) is not documented in the sources I read.

## Fasce gol, soglie and correction rules

### Takeaway
66 points = 1 goal is near-universal (96% of FCO rulebooks). Later goals come every 6 points in the simple scheme (Fantamaster: 66, 72, …) or every 4–6 points; FCO's preset is 66/72/77/81/85/89/93/97/101. Correction rules (draw bonus, superiority, "regola del 59", humiliation) are optional extras that compare the two teams' totals [OPP].

### Cited Findings
- Fantamaster: 1st goal at 66, then one more every 6 points (72, …). Example: 63 vs 65.5 = 0-0. — [Fantamaster regole](https://www.fantamaster.it/regole-fantacalcio-regolamento-asta-moduli-bonus-malus/)
- FCO default 12 thresholds: 1=66, 2=72, 3=77, 4=81, 5=85, 6=89, 7=93, 8=97, 9=101, 10=130.5, 11=134.5, 12=138.5. Below 66 = 0 goals. 66 is the first threshold in 96% of rulebooks. — [FCO fasce gol / regola del 59](https://www.fantacalcio-online.com/it/regole/come-funziona-regola-del-59-bonus-pareggio/amp)
- Fantacalcio.it guideline: universal goal threshold of 66; recommended band width of 4 to 6 points. Optional corrections: "limita vittoria", "limita pareggio", and "autogol" (example: under 55 points the opponent gets an extra goal). — [Fantacalcio.it leghe private](https://www.fantacalcio.it/regolamenti/leghe-private)
- Bonus Pareggio [OPP]: if the teams are in different bands but the gap is below X (most common X = 3, used by 28%), the lower team is raised to a draw. Example: 66.42 vs 65.34 → 1-1. — [FCO](https://www.fantacalcio-online.com/it/regole/come-funziona-regola-del-59-bonus-pareggio/amp)
- Bonus Superiorità [OPP]: same band, gap of at least X (most common 4, used by 31%) → +1 goal to the leader, subject to a minimum score (most common 59). Example: 62 vs 58 → 1-0. — [FCO](https://www.fantacalcio-online.com/it/regole/come-funziona-regola-del-59-bonus-pareggio/amp)
- Regola del 59 [OPP]: only for 0-0 results; the side ahead by at least 3 gets 1 goal, with no minimum (5% usage). Bonus Umiliazione: +1 extra goal when the gap is at least X, any band (most common 10; 5% usage). Superiority and humiliation can stack. — [FCO](https://www.fantacalcio-online.com/it/regole/come-funziona-regola-del-59-bonus-pareggio/amp)
- Fantamaster has equivalent settings: a "soglia pareggio/vittoria" configurable from 0.5 to 5, including a variant for when both scores are under the 1-goal threshold, and "fasce gol valide tra fasce diverse". — [Fantamaster guide](https://www.fantamaster.it/guide-leghe-fantacalcio-fantamaster/category/bonus-e-malus/)

### Inferences
- Goal conversion = monotone threshold list + ordered post-processing rules, each a predicate on (scoreA, scoreB, goalsA, goalsB).

### Gaps
- "Intertoto"-style or minimum-goal-gap rules under those names were not found in the sources I read.

## Fattore campo (home bonus)

### Takeaway
Fattore campo is a fixed number of points added to the home team's total before goal conversion [CAL]. It only applies in calendar (head-to-head) competitions. Fantacalcio.it recommends +2; Fantamaster's base rules list +1. It can be toggled per competition and per matchday.

### Cited Findings
- Fantacalcio.it guideline: home bonus recommended at +2; it does not apply to Battle Royale. — [Fantacalcio.it leghe private](https://www.fantacalcio.it/regolamenti/leghe-private)
- Fantamaster base rules list fattore campo +1. It can be enabled separately for Campionato and Coppa ("Fattore Campo in Campionato" and a cup equivalent). — [Fantamaster regole](https://www.fantamaster.it/regole-fantacalcio-regolamento-asta-moduli-bonus-malus/); [Fantamaster guide](https://www.fantamaster.it/guide-leghe-fantacalcio-fantamaster/category/bonus-e-malus/)
- FCO: applied before conversion, only on matchdays enabled in the calendar, and only in "Campionato" competitions. In odd-sized groups the last round is played on a neutral field. No default value is published. — [FCO fasce gol](https://www.fantacalcio-online.com/it/regole/come-funziona-regola-del-59-bonus-pareggio/amp); [FCO gestione competizioni](https://www.fantacalcio-online.com/it/gestione-leghe/gestione-competizioni/amp)

### Gaps
- No platform-wide default value was found for FCO. Sources conflict on the "typical" value (+1 vs +2).

## Competition formats and tie-breakers

### Takeaway
There are five archetypes: head-to-head league (3-1-0, calendar, andata/ritorno); total points / "tutti contro tutti" with no calendar; Formula 1 (points by daily ranking, configurable table); knockout cup; and groups plus knockout. Fantacalcio.it also lists Old Champions, Tabellone, Battle Royale and Highlander. Competitions share the roster and market but each has its own scoring rules.

### Cited Findings
- FCO formats and usage: Campionato (H2H, andata e ritorno or solo andata, ~72%), Coppa (knockout, with or without return legs, ~20%), Gironi + eliminazione (<1%; choose number of groups and how many qualify), Tutti contro tutti (no calendar, most fantapoints wins, ~35%), Formula 1 (points by position, ~5%). About 1 in 3 leagues runs 2 or more competitions; campionato plus coppa is the most common pairing. The squad and market are league-level and shared. Free plan = 1 competition, PRO = unlimited. — [FCO gestione competizioni](https://www.fantacalcio-online.com/it/gestione-leghe/gestione-competizioni/amp)
- Odd number of teams: a "squadra fantasma" with a preset score, a zero score, or a rest round. — [FCO gestione competizioni](https://www.fantacalcio-online.com/it/gestione-leghe/gestione-competizioni/amp); Fantacalcio.it gives the choice of a rest round or a phantom team — [leghe private](https://www.fantacalcio.it/regolamenti/leghe-private)
- Fantacalcio.it competition types: Campionato, Old Champions, Tabellone, Somma Punti, Formula 1, Battle Royale, Highlander. H2H tie-breakers suggested: total fantapunti, goals scored, goals conceded, goal difference, classifica avulsa. For 1-vs-all or F1: number of matchdays won. Someone joining a Somma Punti league late: 66 points per matchday already played. Postponed matches: wait for the replay or assign a "6 politico" (recommended, especially in cups). — [Fantacalcio.it leghe private](https://www.fantacalcio.it/regolamenti/leghe-private)
- Fantamaster H2H: win 3, draw 1, loss 0, on a Serie A-style calendar. Fantamaster calls the everyone-vs-everyone points-summed format "Formula 1" in its base rules. Its F1 competition guide, however, describes points by daily ranking, configurable per position (example 8 teams: 8…1; illustrative 10/9/8). Other competition types: Punti, Campionato, Coppa con Gruppi, Coppa con Play Off. — [Fantamaster regole](https://www.fantamaster.it/regole-fantacalcio-regolamento-asta-moduli-bonus-malus/); [Fantamaster F1](https://www.fantamaster.it/guide-leghe-fantacalcio-fantamaster/cose-la-competizione-formula-1-come-funziona/)
- FCO F1: the admin sets points for the top 10, with extra bands for 11th–15th and 16th–20th. — [FCO F1/Gran Premio](https://www.fantacalcio-online.com/it/regole/competizioni-f1-granpremio-come-funziona/amp)
- Example house-rule tie-breakers (Mattocalcio league, anecdotal): league points, then total points, goals scored, goals conceded, head-to-head, goal difference. — [Mattocalcio regolamento 2021/22 PDF](https://mattocalcio.altervista.org/wp-content/uploads/2021/08/Regolamento-Lega-Mattocalcio-2021-22.pdf) (older)

### Gaps
- Cup tie-breakers (extra time, penalties, away goals) are in a separate FCO article I did not read.
- No standard default F1 points table (e.g. 25/18/15) was found.

## Line-ups and substitutions (Classic and Mantra)

### Takeaway
Classic uses 11 starters and 7 formations (3-4-3, 3-5-2, 4-5-1, 4-4-2, 4-3-3, 5-4-1, 5-3-2), plus optional emergency formations 3-6-1 and 6-3-1. Bench size and the number of substitutions are configurable. Substitutions are role-for-role, optionally with a change of formation. Mantra has a 12-player bench, 11 formations, a −1 malus per out-of-position player, and block-wise substitution in 3 modes (Basic, Easy, Master).

### Cited Findings
- Classic formations: 3-4-3, 3-5-2, 4-5-1, 4-4-2, 4-3-3, 5-4-1, 5-3-2. — [Fantamaster regole](https://www.fantamaster.it/regole-fantacalcio-regolamento-asta-moduli-bonus-malus/); Fantacalcio.it gives the same list plus emergency 3-6-1 and 6-3-1 — [leghe private](https://www.fantacalcio.it/regolamenti/leghe-private)
- Fantacalcio.it Classic substitution modes: Dinamic (change of formation has priority over role-for-role), Hybrid (change of formation only if no like-for-like sub exists), Traditional (role-for-role only). Recommended minimum is 5 substitutions; a 7-player bench is "too short". Optional "riserva d'ufficio" (example vote 4, GK 2) to reach 11 votes. Booked player without a vote: substituted or given 5.5. "Switch Plus" can be turned on. — [Fantacalcio.it leghe private](https://www.fantacalcio.it/regolamenti/leghe-private)
- Missing line-up options: reuse the last valid line-up (favoured), a "punteggio politico", or a forfeit loss. Deadline recommendation: 5 minutes before the first match. — [Fantacalcio.it leghe private](https://www.fantacalcio.it/regolamenti/leghe-private). Fantamaster deadline: 15 minutes before the first match — [Fantamaster regole](https://www.fantamaster.it/regole-fantacalcio-regolamento-asta-moduli-bonus-malus/)
- FCO: same-role sub first. If change of formation is enabled (60% of rulebooks), the first bench player with a vote who produces a valid formation comes in. Preloaded bench = 11 places. Example of 3 subs, with a suggestion to go from 3 to 4 above 14 teams. A per-slot role constraint (e.g. 1P/2D/2C/2A) is a PLUS feature. Riserva d'ufficio comes last and counts as a substitution; most common values are GK 3 (59%) and outfield 5 (40%) or 4 (27%); usually 1 per matchday (62%); disabled (empty slot = 0) in about 1/3 of leagues. — [FCO riserve d'ufficio](https://www.fantacalcio-online.com/it/regole/come-funzionano-riserve-dufficio)
- Fantamaster 2026/27 has automatic subs, "Swap" and "Sostituzioni Live". — [Fantamaster guide](https://www.fantamaster.it/guide-leghe-fantacalcio-fantamaster/category/bonus-e-malus/)
- Mantra roles: Por, Dc, B, Dd, Ds, E, M, C, T, W, A, Pc (B = full-back usable in a back three). Roles are assigned in late July and fixed for the season. 11 formations, each with 5 defensive outfield slots (Dd/Ds/Dc/B/E/M) and 5 offensive ones (C/T/W/A/Pc). Bench of 12 with at least 1 GK, in absolute preference order. GK is substituted first. Outfield subs are made in a block; candidate sets are tried in bench-order combinatorial order (ABC, ABD, ABE, …). Modes: BASIC (default: optimal in chosen formation → efficient with change of formation, no malus → adapted with malus); EASY (no change of formation); MASTER (any formation with no malus first, then adapted). Malus = −1 per out-of-position player. Forbidden placements: B/Dd/Ds in Dc; Dd↔Ds; E in M; M in E; W in T; in 4-1-4-1, W↔T is not allowed even with malus. If there are too few players, the team plays a man down. — [Fantacalcio.it sistema Mantra](https://www.fantacalcio.it/regolamenti/sistema-mantra)
- Mantra formations: 3-4-3, 3-4-1-2, 3-4-2-1, 3-5-2, 3-5-1-1, 4-3-3, 4-3-1-2, 4-4-2, 4-1-4-1, 4-4-1-1, 4-2-3-1. — [SOS Fanta Mantra](https://www.sosfanta.com/regole-fantacalcio/fantacalcio-mantra-come-funziona-regole-e-differenze-con-il-classic/amp/)
- Conflicting claim: Goal.com lists graded malus values (−1 up to −4, e.g. a forward used in defence). This conflicts with the official flat −1. — [Goal.com](https://www.goal.com/it/liste/fantacalcio-idee-regole-nuove-lega/bltec3ec477596b4a6e) (house-rule idea)

### Gaps
- The per-formation Mantra slot grid is only published as an image or PDF; not extracted.

## Roster and market

### Takeaway
Classic standard: 25 players = 3 P / 8 D / 8 C / 6 A. The budget is typically 500 credits for a free auction among friends, or 250/280 when using official quotazioni (Fantacalcio.it). Mantra is usually 25–32 players with at least 2 GKs. Market sessions, sealed bids, draft, trades and release refunds are all league-configurable.

### Cited Findings
- 25 players: 3 GK, 8 D, 8 C, 6 A. Auctions are open bidding or sealed bid ("busta chiusa"); some leagues start every player at 1 credit. — [Fantamaster regole](https://www.fantamaster.it/regole-fantacalcio-regolamento-asta-moduli-bonus-malus/); 500-credit 3/8/8/6 example — [Calciodangolo](https://calciodangolo.com/fantacalcio-2024-25-rosa-ideale-comporre-500-crediti/) (2024/25)
- Fantacalcio.it Leghe limits. Classic: minimum 17 (1/6/6/4), maximum 90 (15 P, 25 per outfield role). Mantra: minimum 23 with 2 GKs, maximum 90; 28–30 recommended. Credits with official prices: 250 for 25 players (Classic) or 280 for 28 (Mantra); free amount for an auction among friends. Market types: offline auctions, quotation markets, Asta Smart, buste chiuse. A purchase cap per session is required unless set to unlimited. "Asteriscati" (players who left Serie A) can be replaced outside the cap. Svincolo refunds are league-defined. Trades are always allowed to overlap other sessions. Up to 5 divisions. — [Fantacalcio.it leghe private](https://www.fantacalcio.it/regolamenti/leghe-private)
- FCO draft: sealed-bid draft with 3 initial auction rounds; edits allowed until 22:00. — [FCO asta draft](https://www.fantacalcio-online.com/it/gestione-leghe/asta-online-draft-come-funziona/amp). Fantacalcio.it Fanta Asta Live draft: from round 2, the order is by provisional roster value (whoever spent least picks first). — [Fantacalcio.it](https://www.fantacalcio.it/mondo-euroleghe/01_09_2026/asta-draft-con-euroleghe-prova-fanta-asta-live-497335)
- FCO claims about 150 configurable options. — [FCO creazione lega](https://www.fantacalcio-online.com/it/gestione-leghe/creazione-lega)

### Gaps
- No primary source found for keeper/dynasty rules, foreign-player limits, or Fantadraft as a platform. The Gazzetta Fantacampionato official regolamento was not retrievable via search.
