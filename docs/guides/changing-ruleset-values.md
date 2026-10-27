---
title: Changing bonus and malus values
description: Publish a new rule set version and recompute.
---

The values are data, so changing them needs no code change. See [Rule sets](../rulesets.md) for every key.

1. Copy `rulesets/classic.yaml`, change the values, and **bump `version`**. A published version
   is immutable, and the server refuses to start if its content changed.
2. Put the file in `rulesets/` (built into the binary) or in `OPENFANTA_RULESETS_DIR`.
3. Run `openfanta recompute --reason "classic 1.1.0"`.
