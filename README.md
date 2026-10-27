# OpenFanta

OpenFanta is an open fantacalcio data framework. It ingests raw football data from a
third-party provider, computes voti and fantavoti from configurable rule sets, and serves
them over a read-only HTTP API that any client can consume.

> Status: early development. See [docs/design.md](docs/design.md) for the architecture.

## Quick start

```sh
nix develop      # or: direnv allow
make test
```

More (running, adding a provider, changing rules) will be documented as the pieces land.
