# Run targets inside `nix develop` (or with direnv) so tool versions are pinned.
GO        ?= go
PKGS      := ./...
CORE_PKGS := ./internal/domain/... ./internal/rules/... ./internal/settings/... ./internal/timeline/... ./internal/engine/...
CORE_MIN_COVERAGE := 95

.PHONY: docs-dev docs-build all build test test-all test-core cover lint fmt-check gen run db-up db-down pg-start pg-stop

all: lint test build

build:
	$(GO) build -o bin/openfanta ./cmd/openfanta

# Database tests are skipped unless OPENFANTA_TEST_DATABASE_URL is set.
test:
	$(GO) test -race -count=1 $(PKGS)

# Runs every test, database included, against a throwaway local Postgres.
test-all: pg-start
	OPENFANTA_TEST_DATABASE_URL=$$(scripts/dev-postgres.sh url) $(GO) test -race -count=1 $(PKGS)

# Fails if the pure core drops below the coverage gate.
test-core:
	$(GO) test -count=1 -coverprofile=coverage.out $(CORE_PKGS)
	@$(GO) tool cover -func=coverage.out | awk '/^total:/ { sub("%","",$$3); if ($$3+0 < $(CORE_MIN_COVERAGE)) { print "core coverage " $$3 "% < $(CORE_MIN_COVERAGE)%"; exit 1 } else { print "core coverage " $$3 "%" } }'

cover:
	$(GO) test -coverprofile=coverage.out $(PKGS)
	$(GO) tool cover -html=coverage.out -o coverage.html

lint: fmt-check
	golangci-lint run

fmt-check:
	@test -z "$$(gofmt -l .)" || { gofmt -l .; echo "run gofmt -w ."; exit 1; }

gen:
	$(GO) generate ./...

run: build
	./bin/openfanta serve

db-up:
	docker compose up -d db

db-down:
	docker compose down

# Postgres from the Nix devshell, no Docker needed. Data lives in .pgdata/.
pg-start:
	@scripts/dev-postgres.sh start

pg-stop:
	scripts/dev-postgres.sh stop

# Documentation site (Astro Starlight). Content is docs/ and api/openapi.yaml.
docs-dev:
	cd website && npm ci && npm run dev

docs-build:
	cd website && npm ci && npm run build
