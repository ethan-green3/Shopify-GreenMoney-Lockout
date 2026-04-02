package dbmigrate

import (
	"database/sql/driver"
	"os"
	"path/filepath"
	"testing"

	"Shopify-GreenMoney-Lockout/internal/testsql"
)

func TestRunAppliesPendingMigrationsInOrder(t *testing.T) {
	dir := t.TempDir()
	writeMigration(t, dir, "001_first.sql", "CREATE TABLE test_one(id INT);")
	writeMigration(t, dir, "002_second.sql", "CREATE TABLE test_two(id INT);")

	db, state, err := testsql.Open([]testsql.Expectation{
		{Kind: "exec", QueryContains: "CREATE TABLE IF NOT EXISTS schema_migrations", RowsAffected: 0},
		{
			Kind:          "query",
			QueryContains: "FROM schema_migrations",
			Args:          []any{"001_first.sql"},
			Columns:       []string{"exists"},
			Rows:          [][]driver.Value{{false}},
		},
		{Kind: "exec", QueryContains: "CREATE TABLE test_one", RowsAffected: 0},
		{Kind: "exec", QueryContains: "INSERT INTO schema_migrations", Args: []any{"001_first.sql"}, RowsAffected: 1},
		{
			Kind:          "query",
			QueryContains: "FROM schema_migrations",
			Args:          []any{"002_second.sql"},
			Columns:       []string{"exists"},
			Rows:          [][]driver.Value{{false}},
		},
		{Kind: "exec", QueryContains: "CREATE TABLE test_two", RowsAffected: 0},
		{Kind: "exec", QueryContains: "INSERT INTO schema_migrations", Args: []any{"002_second.sql"}, RowsAffected: 1},
	})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	defer db.Close()

	if err := Run(db, dir); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if err := state.Verify(); err != nil {
		t.Fatalf("db expectations not met: %v", err)
	}
}

func TestRunSkipsAppliedMigrations(t *testing.T) {
	dir := t.TempDir()
	writeMigration(t, dir, "001_first.sql", "CREATE TABLE test_one(id INT);")

	db, state, err := testsql.Open([]testsql.Expectation{
		{Kind: "exec", QueryContains: "CREATE TABLE IF NOT EXISTS schema_migrations", RowsAffected: 0},
		{
			Kind:          "query",
			QueryContains: "FROM schema_migrations",
			Args:          []any{"001_first.sql"},
			Columns:       []string{"exists"},
			Rows:          [][]driver.Value{{true}},
		},
	})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	defer db.Close()

	if err := Run(db, dir); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if err := state.Verify(); err != nil {
		t.Fatalf("db expectations not met: %v", err)
	}
}

func writeMigration(t *testing.T, dir, name, contents string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write migration %s: %v", name, err)
	}
}
