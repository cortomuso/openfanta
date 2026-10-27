# Run targets inside `nix develop` (or with direnv) so tool versions are pinned.
GO        ?= go
PKGS      := ./...
CORE_PKGS := ./internal/domain/... ./internal/rules/... ./internal/timeline/... ./internal/engine/...
CORE_MIN_COVERAGE := 95

.PHONY: all build test test-core cover lint gen run db-up db-down

all: lint test build

build:
	$(GO) build -o bin/openfanta ./cmd/openfanta

test:
	$(GO) test -race -count=1 $(PKGS)

# Fails if the pure core drops below the coverage gate.
test-core:
	$(GO) test -count=1 -coverprofile=coverage.out $(CORE_PKGS)
	@$(GO) tool cover -func=coverage.out | awk '/^total:/ { sub("%","",$$3); if ($$3+0 < $(CORE_MIN_COVERAGE)) { print "core coverage " $$3 "% < $(CORE_MIN_COVERAGE)%"; exit 1 } else { print "core coverage " $$3 "%" } }'

cover:
	$(GO) test -coverprofile=coverage.out $(PKGS)
	$(GO) tool cover -html=coverage.out -o coverage.html

lint:
	golangci-lint run

gen:
	$(GO) generate ./...

run: build
	./bin/openfanta serve

db-up:
	docker compose up -d db

db-down:
	docker compose down
