// Package storetest provisions a fresh, migrated Postgres database per test.
//
// Tests using it are skipped unless OPENFANTA_TEST_DATABASE_URL points to a
// server where the user may create databases (see scripts/dev-postgres.sh).
package storetest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/openfanta/openfanta/internal/store"
)

// EnvVar names the admin connection URL used to create test databases.
const EnvVar = "OPENFANTA_TEST_DATABASE_URL"

// New creates a migrated database, opens a Store on it, and drops it on cleanup.
func New(t *testing.T) (*store.Store, string) {
	t.Helper()
	admin := os.Getenv(EnvVar)
	if admin == "" {
		t.Skipf("%s not set; skipping database test", EnvVar)
	}
	ctx := context.Background()

	suffix := make([]byte, 6)
	_, _ = rand.Read(suffix)
	name := "openfanta_test_" + hex.EncodeToString(suffix)

	conn, err := pgx.Connect(ctx, admin)
	if err != nil {
		t.Fatalf("connect admin: %v", err)
	}
	defer func() { _ = conn.Close(ctx) }()
	if _, err := conn.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatalf("create database: %v", err)
	}

	u, err := url.Parse(admin)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name
	dsn := u.String()

	if err := store.Migrate(ctx, dsn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	s, err := store.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		s.Close()
		c, err := pgx.Connect(ctx, admin)
		if err != nil {
			t.Logf("cleanup connect: %v", err)
			return
		}
		defer func() { _ = c.Close(ctx) }()
		if _, err := c.Exec(ctx, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)"); err != nil {
			t.Logf("drop database: %v", err)
		}
	})
	return s, dsn
}
