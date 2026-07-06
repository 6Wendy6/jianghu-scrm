package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	command := os.Args[1]
	flags := flag.NewFlagSet(command, flag.ExitOnError)
	databaseURL := flags.String("database-url", firstNonEmpty(os.Getenv("SCRM_DATABASE_URL"), os.Getenv("DATABASE_URL")), "PostgreSQL connection string")
	migrationsDir := flags.String("migrations", "migrations", "migration directory")
	seedFile := flags.String("seed", "seeds/customer_ops_demo_seed.sql", "seed SQL file")
	timeout := flags.Duration("timeout", 30*time.Second, "operation timeout")
	_ = flags.Parse(os.Args[2:])
	if strings.TrimSpace(*databaseURL) == "" {
		fmt.Fprintln(os.Stderr, "database URL is required; set SCRM_DATABASE_URL or DATABASE_URL")
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	db, err := sql.Open("postgres", *databaseURL)
	if err != nil {
		exitErr(err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		exitErr(fmt.Errorf("ping database: %w", err))
	}
	switch command {
	case "migrate":
		exitErr(runMigrations(ctx, db, *migrationsDir))
		fmt.Printf("migrations applied from %s\n", *migrationsDir)
	case "seed":
		exitErr(runSQLFile(ctx, db, *seedFile))
		fmt.Printf("seed applied from %s\n", *seedFile)
	case "status":
		version, _ := latestAppliedMigration(ctx, db)
		fmt.Printf("database connected=true latestAppliedMigration=%s latestKnownMigration=%s\n", version, latestKnownMigration(*migrationsDir))
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: scrm-admin <migrate|seed|status> [--database-url URL] [--migrations DIR] [--seed FILE]")
}

func exitErr(err error) {
	if err == nil {
		return
	}
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func runMigrations(ctx context.Context, db *sql.DB, dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	sort.Strings(files)
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			checksum TEXT NOT NULL DEFAULT '',
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`); err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}
	for _, file := range files {
		content, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", file, err)
		}
		if strings.TrimSpace(string(content)) == "" {
			continue
		}
		if _, err := db.ExecContext(ctx, string(content)); err != nil {
			return fmt.Errorf("run migration %s: %w", file, err)
		}
		sum := sha256.Sum256(content)
		if _, err := db.ExecContext(ctx, `
			INSERT INTO schema_migrations (version, checksum, applied_at)
			VALUES ($1, $2, now())
			ON CONFLICT (version) DO UPDATE SET checksum = EXCLUDED.checksum, applied_at = now()
		`, filepath.Base(file), hex.EncodeToString(sum[:])); err != nil {
			return fmt.Errorf("record migration %s: %w", file, err)
		}
	}
	return nil
}

func runSQLFile(ctx context.Context, db *sql.DB, file string) error {
	content, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("read sql file %s: %w", file, err)
	}
	if strings.TrimSpace(string(content)) == "" {
		return nil
	}
	if _, err := db.ExecContext(ctx, string(content)); err != nil {
		return fmt.Errorf("run sql file %s: %w", file, err)
	}
	return nil
}

func latestAppliedMigration(ctx context.Context, db *sql.DB) (string, error) {
	var version string
	err := db.QueryRowContext(ctx, `SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1`).Scan(&version)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return version, err
}

func latestKnownMigration(dir string) string {
	files, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil || len(files) == 0 {
		return ""
	}
	sort.Strings(files)
	return filepath.Base(files[len(files)-1])
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
