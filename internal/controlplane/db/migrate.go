package db

import (
	"embed"
	"errors"
	"fmt"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type migrationRunner interface {
	Up() error
	Close() (error, error)
}

var newMigrationSource = func() (source.Driver, error) {
	return iofs.New(migrationsFS, "migrations")
}

var newMigrationRunner = func(
	src source.Driver,
	databaseURL string,
) (migrationRunner, error) {
	return migrate.NewWithSourceInstance("iofs", src, toPgxURL(databaseURL))
}

// Migrate brings the database schema to the latest version using the embedded
// SQL files. Safe to call on every boot — no-op if already at head.
func Migrate(databaseURL string) error {
	src, err := newMigrationSource()
	if err != nil {
		return fmt.Errorf("load migration source: %w", err)
	}

	m, err := newMigrationRunner(src, databaseURL)
	if err != nil {
		return fmt.Errorf("init migrator: %w", err)
	}
	defer func() { _, _ = m.Close() }()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

// toPgxURL rewrites a postgres:// URL to use the pgx5 migrate driver.
func toPgxURL(url string) string {
	if strings.HasPrefix(url, "postgres://") {
		return "pgx5://" + strings.TrimPrefix(url, "postgres://")
	}
	if strings.HasPrefix(url, "postgresql://") {
		return "pgx5://" + strings.TrimPrefix(url, "postgresql://")
	}
	return url
}
