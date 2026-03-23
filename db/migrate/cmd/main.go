package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	if err := run(logger); err != nil {
		logger.Error("migrate failed", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	databaseURL := buildDatabaseURL()
	migrationsPath := migrationsDir()
	command := migrationCommand()

	logger.Info("running migrations",
		"command", command,
		"migrations", migrationsPath,
		"host", os.Getenv("POSTGRES_HOST"),
		"database", os.Getenv("POSTGRES_DB"),
	)

	m, err := migrate.New("file://"+migrationsPath, databaseURL)
	if err != nil {
		return fmt.Errorf("creating migrator: %w", err)
	}
	defer m.Close()

	switch command {
	case "up":
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			return fmt.Errorf("running up migrations: %w", err)
		}
		version, dirty, _ := m.Version()
		logger.Info("migrations applied", "version", version, "dirty", dirty)

	case "down":
		if err := m.Steps(-1); err != nil && err != migrate.ErrNoChange {
			return fmt.Errorf("rolling back migration: %w", err)
		}
		version, dirty, _ := m.Version()
		logger.Info("migration rolled back", "version", version, "dirty", dirty)

	case "version":
		version, dirty, err := m.Version()
		if err != nil {
			return fmt.Errorf("getting migration version: %w", err)
		}
		logger.Info("current version", "version", version, "dirty", dirty)

	default:
		return fmt.Errorf("unknown command %q — use: up, down, version", command)
	}

	return nil
}

func buildDatabaseURL() string {
	host := getenv("POSTGRES_HOST", "localhost")
	port := getenv("POSTGRES_PORT", "5432")
	user := getenv("POSTGRES_USER", "jobradar")
	password := getenv("POSTGRES_PASSWORD", "")
	database := getenv("POSTGRES_DB", "jobradar")
	sslmode := getenv("POSTGRES_SSL_MODE", "disable")

	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		user, password, host, port, database, sslmode,
	)
}

func migrationsDir() string {
	return getenv("MIGRATIONS_PATH", "/migrations")
}

func migrationCommand() string {
	if len(os.Args) > 1 {
		return os.Args[1]
	}
	return getenv("MIGRATE_COMMAND", "up")
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
