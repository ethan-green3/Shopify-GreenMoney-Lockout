package dbmigrate

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const schemaMigrationsTable = "schema_migrations"

func Run(db *sql.DB, dir string) error {
	if strings.TrimSpace(dir) == "" {
		return fmt.Errorf("migrations directory is required")
	}

	if err := ensureSchemaMigrationsTable(db); err != nil {
		return err
	}

	files, err := migrationFiles(dir)
	if err != nil {
		return err
	}

	for _, file := range files {
		version := filepath.Base(file)
		applied, err := isApplied(db, version)
		if err != nil {
			return err
		}
		if applied {
			continue
		}

		contents, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", file, err)
		}

		if err := applyMigration(db, version, string(contents)); err != nil {
			return fmt.Errorf("apply migration %s: %w", version, err)
		}
	}

	return nil
}

func ensureSchemaMigrationsTable(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("ensure schema_migrations table: %w", err)
	}
	return nil
}

func migrationFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations dir %s: %w", dir, err)
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		files = append(files, filepath.Join(dir, entry.Name()))
	}
	sort.Strings(files)
	return files, nil
}

func isApplied(db *sql.DB, version string) (bool, error) {
	var exists bool
	err := db.QueryRow(`
		SELECT EXISTS (
			SELECT 1
			FROM schema_migrations
			WHERE version = $1
		)
	`, version).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check migration %s: %w", version, err)
	}
	return exists, nil
}

func applyMigration(db *sql.DB, version, sqlText string) error {
	if _, err := db.Exec(sqlText); err != nil {
		return fmt.Errorf("exec migration SQL: %w", err)
	}
	if _, err := db.Exec(`INSERT INTO schema_migrations (version) VALUES ($1)`, version); err != nil {
		return fmt.Errorf("record migration: %w", err)
	}
	return nil
}
