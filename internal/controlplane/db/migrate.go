package db

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strconv"
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
	Version() (uint, bool, error)
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

type MigrationStatus struct {
	CurrentVersion uint `json:"current_version"`
	HeadVersion    uint `json:"head_version"`
	Dirty          bool `json:"dirty"`
	AtHead         bool `json:"at_head"`
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

func MigrationState(databaseURL string) (MigrationStatus, error) {
	headVersion, err := latestEmbeddedMigrationVersion()
	if err != nil {
		return MigrationStatus{}, fmt.Errorf("discover migration head: %w", err)
	}

	src, err := newMigrationSource()
	if err != nil {
		return MigrationStatus{}, fmt.Errorf("load migration source: %w", err)
	}

	m, err := newMigrationRunner(src, databaseURL)
	if err != nil {
		return MigrationStatus{}, fmt.Errorf("init migrator: %w", err)
	}
	defer func() { _, _ = m.Close() }()

	currentVersion, dirty, err := m.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return MigrationStatus{}, fmt.Errorf("read migration version: %w", err)
	}
	if errors.Is(err, migrate.ErrNilVersion) {
		currentVersion = 0
		dirty = false
	}

	return MigrationStatus{
		CurrentVersion: currentVersion,
		HeadVersion:    headVersion,
		Dirty:          dirty,
		AtHead:         !dirty && currentVersion == headVersion,
	}, nil
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

func latestEmbeddedMigrationVersion() (uint, error) {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return 0, err
	}

	var latest uint
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		version, ok, err := migrationVersionFromFilename(entry.Name())
		if err != nil {
			return 0, err
		}
		if ok && version > latest {
			latest = version
		}
	}
	return latest, nil
}

func migrationVersionFromFilename(name string) (uint, bool, error) {
	base := filepath.Base(name)
	if !strings.HasSuffix(base, ".up.sql") {
		return 0, false, nil
	}
	versionPart, _, ok := strings.Cut(base, "_")
	if !ok {
		return 0, false, fmt.Errorf("invalid migration filename %q", name)
	}
	version, err := strconv.ParseUint(versionPart, 10, 32)
	if err != nil {
		return 0, false, fmt.Errorf("parse migration version from %q: %w", name, err)
	}
	return uint(version), true, nil
}
