---
title: Changing the default scoring values
description: Edit the Classic default and recompute.
---

Until leagues score under their own settings, every fixture is scored under the Classic default,
[`internal/settings/classic.json`](https://github.com/cortomuso/openfanta/blob/main/internal/settings/classic.json).
See [League settings](../settings.md) for what each part does.

1. Edit the values in `classic.json`. Points are integer hundredths.
2. Run `make test`. `settings.Classic()` panics at start-up, and the tests fail, if the document is invalid.
3. Run `openfanta recompute --reason "goal 3.5"`.

The changed document has a new hash, so the recompute adds a run under the new hash. Runs under
the old hash stay in the history. The API serves the runs of the current default.
