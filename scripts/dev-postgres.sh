#!/usr/bin/env bash
# Run a throwaway Postgres from the Nix devshell, without Docker.
# Usage: scripts/dev-postgres.sh start|stop|url
set -euo pipefail

DATA="${PGDATA_DIR:-$PWD/.pgdata}"
PORT="${PGPORT:-5433}"
URL="postgres://openfanta@localhost:${PORT}/openfanta?sslmode=disable"

case "${1:-}" in
  start)
    if [ ! -d "$DATA" ]; then
      initdb --auth=trust --username=openfanta --encoding=UTF8 "$DATA" >/dev/null
    fi
    if ! pg_ctl -D "$DATA" status >/dev/null 2>&1; then
      pg_ctl -D "$DATA" -o "-p $PORT -k $DATA -c listen_addresses=localhost" -l "$DATA/log" -w start >/dev/null
    fi
    createdb -h localhost -p "$PORT" -U openfanta openfanta 2>/dev/null || true
    echo "$URL"
    ;;
  stop)
    pg_ctl -D "$DATA" -w stop >/dev/null
    ;;
  url)
    echo "$URL"
    ;;
  *)
    echo "usage: $0 start|stop|url" >&2
    exit 2
    ;;
esac
