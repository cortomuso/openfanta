# Player-level scoring rules in Italian fantacalcio (platforms and house rules)

## Classic vs Mantra: player-level differences

### Takeaway
Bonus/malus values are the same concept in both systems. Classic lets leagues set values per macro-role (P/D/C/A). Mantra only distinguishes goalkeeper vs outfield, and adds a -1 out-of-position malus that depends on lineup context, not on the player's match.

### Cited Findings
- Fantacalcio.it Leghe: bonus/malus can be set per role. In Classic the split is P, D, C, A. In Mantra the split is only goalkeepers vs outfield players — [Fantacalcio.it regolamento leghe private](https://www.fantacalcio.it/regolamenti/leghe-private)
- Classic squad role limits (system min/max): 1/15 P, 6/25 D, 6/25 C, 4/25 A — [Fantacalcio.it leghe private](https://www.fantacalcio.it/regolamenti/leghe-private)
- Mantra roles: Por, Dc, B, Dd, Ds, E, M, C, T, W, A, Pc. Players can hold several roles — [Fantacalcio.it sistema Mantra](https://www.fantacalcio.it/regolamenti/sistema-mantra)
- Mantra out-of-position malus: "aggravio di un malus di 1 punto" for each player fielded out of position. In the initial lineup, some placements are banned outright: B/Dd/Ds in Dc; Dd in Ds and vice versa; E in M (allowed in M/C); M in E (allowed in E/W); W in T (allowed in T/A). The same placements are allowed during automatic substitutions, with the malus. In 4-1-4-1, W and T can't replace each other, even with the malus — [Fantacalcio.it sistema Mantra](https://www.fantacalcio.it/regolamenti/sistema-mantra)
- Mantra substitution modes BASIC (default) / EASY / MASTER decide whether the formation can change and when a malus is accepted. Bench of 12 with at least one GK. If the number of substitutions is limited, a reserve GK coming in uses one of them — [Fantacalcio.it sistema Mantra](https://www.fantacalcio.it/regolamenti/sistema-mantra)
- Mantra roles and quotes come from a single source (Fantacalcio). Classic uses the Fantacalcio role list by default, with an editor for borderline cases — [Fantacalcio.it leghe private](https://www.fantacalcio.it/regolamenti/leghe-private)
- Mantra has the team-level D-Factor and R-Factor. They are optional and can't be used together. No numbers are published on the page — [Fantacalcio.it sistema Mantra](https://www.fantacalcio.it/regolamenti/sistema-mantra)
- Mantra rosters need at least 2 GKs in a minimum roster of 23 — [Fantacalcio.it leghe private](https://www.fantacalcio.it/regolamenti/leghe-private)

### Inferences
- In a settings format, bonus values should be keyed by a role group. Classic uses {P,D,C,A}; Mantra uses {GK, outfield}. A generic map from role to value keeps the format flexible, so it can also hold FCO-style per-role values for any role.
- The out-of-position malus depends on the player's role and the lineup slot. It is lineup-level, not match-level, and is applied after the player's fantavoto is computed. It belongs in lineup/substitution settings rather than in the player bonus table.

### Gaps
- I found no official Mantra table that gives per-role (e.g. Dc vs A) goal values. The official page lists none.
- No published numbers for D-Factor or R-Factor.

## Bonus/malus tables: defaults and configurability

### Takeaway
Most platforms share a core set: goal +3, assist +1, yellow -0.5, red -1, own goal -2, goal conceded -1 (GK), penalty missed -3, penalty saved +3. Fantacalcio.it splits assists into categories (in movimento / da fermo; Soft/Standard/Gold; contributo al gol). Clean sheet and gol decisivo are optional. FCO offers "advanced" event bonuses such as woodwork, penalty won/conceded and decisive error. On these platforms every value is configurable, usually per role.

### Cited Findings
**Fantacalcio.it / Leghe Fantacalcio (recommended "standard" values; the site says no official rulebook exists)** — [Fantacalcio.it leghe private](https://www.fantacalcio.it/regolamenti/leghe-private)
- Goal scored (penalty goals included): +3
- Goal conceded: -1 (goalkeeper category)
- Penalty missed: -3. Penalty saved: +3
- Assist in movimento: +1. Assist da fermo: +0.5 ("some prefer +1")
- Yellow: -0.5. Red: -1
- Optional: own goal -2; porta inviolata (GK) +1; gol decisivo pareggio +1; gol decisivo vittoria +1; Panini Player of the Match +0.5
- Assist categories gold, standard and soft each default to 1. Any of them can be excluded or given a different value. "Contributo al gol" is off by default.

**Fantacalcio.it assist definitions (since 2021/22)**
- An assist is a pass decisive for the goal that both gives the scorer an advantage and is voluntary. The call is made solely by the editorial team. Leagues can enable qualitative assists (Soft/Standard/Gold, values at league discretion, e.g. 0.5/1/1.5). "Contributo al gol" covers passes that meet at least one of the two criteria but were not judged assists. The article recommends valuing it below an assist — [SOS Fanta, 28 Jul 2021](https://www.sosfanta.com/news/rivoluzione-assist-fantacalcio-it-cosa-cambia-leghe-contributo-al-gol/)
- The editorial "Analisi assist" column labels each assist Soft/Standard/Gold for every round — [Fantacalcio.it analisi assist](https://www.fantacalcio.it/analisi-assist/1)

**Cards (Fantacalcio.it rules)**
- Double yellow, direct red, or yellow then red are treated the same. The maximum card malus is -1; "-1.5" doesn't exist. A card received off the pitch counts only if the player entered the match. Unused substitutes are excluded — [Fantacalcio.it regole generali (2020)](https://www.fantacalcio.it/regolamenti/19_06_2020/regolamento-ufficiale-fantacalcio-412003)
- VAR-annulled actions are void, and any rating changes made for them are reverted — [same](https://www.fantacalcio.it/regolamenti/19_06_2020/regolamento-ufficiale-fantacalcio-412003)

**Fantacalcio-Online (FCO) defaults** — [FCO guida completa](https://www.fantacalcio-online.com/it/regole/regolamento-fantacalcio-guida-completa)
- Goal +3; assist +1; penalty scored +2.5 (*differs from Fantacalcio.it, which counts a penalty goal as a normal +3 goal*); penalty saved +2.5; penalty missed -2.5; yellow -0.5; red -1; own goal -2; goal conceded -1 per goal (GK)
- "Il valore di ogni evento è un campo del regolamento, e può essere diverso per ruolo" (each event's value is a league field and can differ by role). The site claims 150+ configurable fields — [FCO guida](https://www.fantacalcio-online.com/it/regole/regolamento-fantacalcio-guida-completa); [FCO differenze Magic](https://www.fantacalcio-online.com/it/regole/differenze-regolamento-magic-gazzetta)
- Goals always come from one official source. The assist source is selectable because editorial teams use different criteria — [FCO differenze Magic](https://www.fantacalcio-online.com/it/regole/differenze-regolamento-magic-gazzetta)

**FCO "advanced" bonus/malus (most-chosen value; share of leagues using it)** — [FCO bonus/malus avanzati](https://www.fantacalcio-online.com/it/regole/bonus-malus-avanzati-fantacalcio-come-funzionano)
- Porta 90' imbattuta +1 (42%); palo/traversa +1 (30%); rigore conquistato +0.5 (29%); salvataggio decisivo +0.5 (3%)
- Rigore concesso -0.5 (28%); errore decisivo (gol) -0.5 (27%); errore decisivo (tiro) -0.1 (3%); gol divorato no value (0%)
- Assist subtypes: assist, assist da fermo, passaggi decisivi (key passes)
- Removed in August 2026: assist su autogol, assist su palo, passaggio sbagliato che porta al gol, and the alternative assist source

**Gazzetta / Magic (OLDER, 2016/17 rule changes reported by SOS Fanta)** — [SOS Fanta, 15 Aug 2016](https://www.sosfanta.com/news/regolamento-cosa-cambia-tra-assist-malus-per-autogol-e-regola-dei-15-minuti/)
- Own goal: -2 for outfield players, -1 for goalkeepers. This is a role-dependent malus.
- Assist definitions: a long-range shot must be a first-time strike; no assist if the scorer dribbles past opponents; a direct corner goal or a goal from a set-piece pass earns an assist; no assist for a square pass to a direct free-kick taker; a deflected pass counts if the deflection doesn't change the goal. On a passed penalty ("Messi" case) the passer gets an assist rather than the -3 missed-penalty malus.
- A later Gazzetta assist tweak (undated): the assist stays +1 and is given if the scorer shoots within three touches — [SOS Fanta](https://www.sosfanta.com/news/gazzetta-cambieranno-gli-assist-ce-la-nuova-regola/)

**Fantamaster (generic guide, 26 Jul 2021)** — [Fantamaster regole](https://www.fantamaster.it/regole-fantacalcio-regolamento-asta-moduli-bonus-malus/)
- Goal +3; assist +1; GK clean sheet +1; penalty saved +3; goal conceded -1; red -1; yellow -0.5; penalty missed -3; own goal -2. Captain bonus and midfield modifier are set in the league's Bonus/Malus section — [Fantamaster guide category](https://www.fantamaster.it/guide-leghe-fantacalcio-fantamaster/category/bonus-e-malus/)

**Setting-by-setting context dependency**
| Setting | Inputs needed |
|---|---|
| Goal, penalty scored/missed, assist (any kind), yellow/red, own goal, penalty saved, woodwork, penalty won/conceded, decisive error | Player's own match events, plus role for per-role values |
| Goal conceded (GK) | Player's events (goals conceded while on the pitch) |
| Porta imbattuta / clean sheet | Goals conceded and minutes played (FCO: "90'") |
| Gol decisivo vittoria/pareggio | Real match result and goal timeline (the goal must decide the win or draw) |
| Panini Player of the Match | External award |
| Mantra out-of-position malus | Lineup slot vs player role (lineup context) |

### Inferences
- Fantacalcio.it and FCO disagree on penalty goals (+3 vs +2.5). The format needs a separate "penalty_scored" value that can either replace the goal bonus or be added to it.
- Gazzetta's GK own goal of -1 shows that per-role values are needed for maluses too, not only for goals.
- Assists should be modelled as a list of typed assist events (movimento/da fermo, soft/standard/gold, contributo, key pass), each with its own value. Sources differ on which types exist.

### Gaps
- No primary 2025/26 or 2026/27 Gazzetta Magic / Fantacampionato rulebook was retrieved; the Gazzetta values above come from secondary sources and are old.
- No sources were found for Fantadraft, Fantaclub, Sky-branded fantacalcio, or FPL-like Italian games.
- "Gol subito su rigore" as a separate malus: no source found.
- I found no source confirming per-role goal values (e.g. defender goal +4.5) as a platform default. Aranzulla says Leghe Fantacalcio lets leagues set different goal bonuses by role, but I didn't verify this against the primary source.

## Voto base: sources and precision

### Takeaway
Leghe Fantacalcio offers three rating sources: Fantacalcio (journalistic, with Voti Live), Voto Statistico FG (algorithm) and Redazione Italia (a synthesis). Roles, ratings and bonus/malus sources are chosen independently. FCO can combine up to three weighted sources and rounds the result to two decimals.

### Cited Findings
- Leghe Fantacalcio rating sources: Fantacalcio (journalistic, with Voti Live); Voto Statistico FG (Alvin482 algorithm); Redazione Italia (a synthesis of several ratings) — [Fantacalcio.it leghe private §6.2](https://www.fantacalcio.it/regolamenti/leghe-private)
- Role, rating and bonus/malus sources are separate choices: "ogni singola voce è indipendente e non vincolante sulle restanti scelte" (each choice is independent of the others) — [Fantacalcio.it leghe private](https://www.fantacalcio.it/regolamenti/leghe-private)
- FCO: the starting rating is a weighted average of the chosen sources, rounded to two decimals. Fantacalcio.it ratings are included in every plan. The Plus plan adds the Roma and Torino editions, which can be used per competition or combined with weights. "Fino a tre fonti" (up to three sources) can be combined — [FCO guida](https://www.fantacalcio-online.com/it/regole/regolamento-fantacalcio-guida-completa); [FCO differenze Magic](https://www.fantacalcio-online.com/it/regole/differenze-regolamento-magic-gazzetta)
- Fantacalcio.it warns that ratings shouldn't be compared across players or matches — [Fantacalcio.it regole generali (2020)](https://www.fantacalcio.it/regolamenti/19_06_2020/regolamento-ufficiale-fantacalcio-412003)

### Inferences
- A rating source should be modelled as a list of {source_id, weight} plus a precision/rounding setting. The rounding setting should cover FCO's two decimals and the classic half-point ratings.

### Gaps
- Fantavoto rounding rules (e.g. whether 6.25 is rounded) weren't found on any platform page.
- The current Gazzetta, Corriere dello Sport and Tuttosport options in Leghe Fantacalcio weren't confirmed for 2025/26. The leghe-private page lists only the three sources above.

## Senza voto (s.v.), voto d'ufficio, postponed and suspended matches

### Takeaway
Fantacalcio.it has no minimum-minutes rule; the journalist decides whether a player gets a rating. An s.v. player sent off during the match gets 4. Any other bonus/malus gives 6 plus that bonus/malus. For a yellow card with s.v., the league chooses between substitution and 5.5. Gazzetta historically used 15 minutes (25 for GKs) and a 6 or 5.5 voto d'ufficio. Postponed matches are configurable: wait for the recovery match, or give a "6 politico" to the whole roster.

### Cited Findings
**Fantacalcio.it (rules dated 2020, still linked)** — [regole generali](https://www.fantacalcio.it/regolamenti/19_06_2020/regolamento-ufficiale-fantacalcio-412003)
- "Non esiste, e mai esisterà, una regola che prevede che il voto venga dato a chi gioca un numero minimo di minuti" (there is not, and never will be, a minimum-minutes rule). A player can be rated after 5 minutes, or get s.v. after 20.
- s.v. and sent off during the match: fantavoto automatically 4. Sent off after the match: stays s.v.
- s.v. with any other bonus/malus: 6 plus the bonus/malus.
- s.v. with a yellow card: the league chooses a normal s.v. (substitution) or a fixed 5.5, which already includes the yellow malus. The same option appears in [leghe private](https://www.fantacalcio.it/regolamenti/leghe-private).
- A GK who plays a few minutes and gets s.v. is substituted, and the clean-sheet bonus is not added.
- Optional riserva d'ufficio (when no substitute is available): the league sets the vote, e.g. 4, with a separate GK value, e.g. 2 — [leghe private](https://www.fantacalcio.it/regolamenti/leghe-private)

**Postponed matches (rinviate), Fantacalcio.it**
- A match played between the end of one round and the start of the next is not treated as postponed. Otherwise the league chooses: wait for the recovery match, count the match if it is brought forward, or give a 6 politico to all players of the teams involved. The 6 politico is the recommended option. It goes to the whole roster (reserves, injured and suspended players included) and counts as a normal rating for modifiers — [regole generali](https://www.fantacalcio.it/regolamenti/19_06_2020/regolamento-ufficiale-fantacalcio-412003); [leghe private §15](https://www.fantacalcio.it/regolamenti/leghe-private)
- Transfers: if a player already played that round for his old team (rated or s.v.), that performance stands. Otherwise the new team's match counts — [regole generali](https://www.fantacalcio.it/regolamenti/19_06_2020/regolamento-ufficiale-fantacalcio-412003)

**Suspended matches (sospese after kickoff), Fantacalcio.it**
- If the restart falls after the next round starts, the league chooses: (1) a 6 politico with no bonus/malus for every player of both teams, including those who didn't play; or (2) wait for the restart, after which bonus/malus from both parts count and up to 14 players per team can be rated. If the restart is before the next round starts, the 6 politico option is unavailable — [regole generali](https://www.fantacalcio.it/regolamenti/19_06_2020/regolamento-ufficiale-fantacalcio-412003)

**Forfeit (tavolino), older rule quoted in 2016**: players with at least 30 minutes get a 6 plus their bonus/malus; other players who entered are treated as s.v. — [FCO Sassuolo-Pescara](https://www.fantacalcio-online.com/it/seriea_news/sassuolo-pescara-tavolino-fantacalcio)

**Gazzetta (2016/17)** — [SOS Fanta 2016](https://www.sosfanta.com/news/regolamento-cosa-cambia-tra-assist-malus-per-autogol-e-regola-dei-15-minuti/)
- Minimum 15 minutes (25 for GKs) for a voto d'ufficio, which applies only when the journalist gives no rating.
- Substituted early but reached 15' (25' GK): 6. Came on then was substituted, with ≥15': 5.5. Came on by the 75th minute and finished: 5.5.
- A bonus/malus other than a yellow card gives 6 plus or minus that bonus/malus.
- GK s.v. who entered (even if later substituted) gets 6, needing ≥25' including stoppage time — [SOS Fanta Donnarumma](https://www.sosfanta.com/news/donnarumma-e-s-v-ecco-il-regolamento-gazzetta/amp/)

**FCO s.v. options**
- The league chooses substitution or a voto d'ufficio (3 to 6) with a minutes threshold. The threshold must be the same for players who came off and players who came on, so Magic's split can't be replicated. About 94% of FCO leagues choose substitution. FCO's objective rating uses 10 minutes for GKs — [FCO minutaggio](https://www.fantacalcio-online.com/it/voti-oggettivi/come-funziona-voto-oggettivo-minutaggio-senza-voto); [FCO differenze Magic](https://www.fantacalcio-online.com/it/regole/differenze-regolamento-magic-gazzetta)
- Postponement on FCO: the default is to wait. Otherwise a per-match voto d'ufficio (5, 5.5, 6, or 6 outfield / 5 GK). It goes to the whole roster including suspended players, with no exclusion or GK differentiation in FCO's implementation — [FCO partite rinviate](https://www.fantacalcio-online.com/it/guide/gestione-leghe/gestione-partite-rinviate); [FCO differenze Magic](https://www.fantacalcio-online.com/it/regole/differenze-regolamento-magic-gazzetta)
- Goal.com: the two main options for a postponed match are a 6 politico or waiting for the recovery match — [Goal.com Bologna-Milan](https://www.goal.com/it/liste/bologna-milan-rinviata-cosa-succede-al-fantacalcio-6-politico-e-tutte-le-opzioni-da-regolamento/blt18669034ba5557c8)

### Inferences
- An s.v. policy needs several fields: the mode (substitute vs voto d'ufficio); an optional minutes threshold, possibly split by GK vs outfield and by subbed-off vs subbed-on (Magic); the base vote when an event occurs (6 + bonus/malus); special cases (red card → 4; yellow → substitute or 5.5); and a GK clean-sheet suppression flag.
- Postponed and suspended handling is per real match. Its inputs are mode (wait / voto d'ufficio / count early), the vote value (GK and outfield), whether bonus/malus apply, and roster scope (who gets it: whole roster vs players who played).

### Gaps
- No current (2025/26+) primary Gazzetta Magic s.v. rules were retrieved. Gazzetta data is from 2016 secondary reporting.
- Reddit/forum house rules were not researched within the tool budget.
