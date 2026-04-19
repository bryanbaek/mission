package db

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/source"
)

type fakeMigrationRunner struct {
	upErr       error
	upCalled    bool
	closeCalled bool
}

func (f *fakeMigrationRunner) Up() error {
	f.upCalled = true
	return f.upErr
}

func (f *fakeMigrationRunner) Close() (error, error) {
	f.closeCalled = true
	return nil, nil
}

type fakeSourceDriver struct{}

func (fakeSourceDriver) Open(string) (source.Driver, error) { return fakeSourceDriver{}, nil }
func (fakeSourceDriver) Close() error                       { return nil }
func (fakeSourceDriver) First() (uint, error)               { return 0, nil }
func (fakeSourceDriver) Prev(uint) (uint, error)            { return 0, nil }
func (fakeSourceDriver) Next(uint) (uint, error)            { return 0, nil }
func (fakeSourceDriver) ReadUp(uint) (io.ReadCloser, string, error) {
	return io.NopCloser(strings.NewReader("")), "", nil
}
func (fakeSourceDriver) ReadDown(uint) (io.ReadCloser, string, error) {
	return io.NopCloser(strings.NewReader("")), "", nil
}

func restoreMigrateSeams(t *testing.T) {
	t.Helper()

	origSource := newMigrationSource
	origRunner := newMigrationRunner
	t.Cleanup(func() {
		newMigrationSource = origSource
		newMigrationRunner = origRunner
	})
}

func TestToPgxURL(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "postgres", in: "postgres://host/db", want: "pgx5://host/db"},
		{name: "postgresql", in: "postgresql://host/db", want: "pgx5://host/db"},
		{name: "already rewritten", in: "pgx5://host/db", want: "pgx5://host/db"},
		{name: "other scheme", in: "mysql://host/db", want: "mysql://host/db"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := toPgxURL(tc.in); got != tc.want {
				t.Fatalf("toPgxURL(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestMigrateSuccess(t *testing.T) {
	restoreMigrateSeams(t)

	runner := &fakeMigrationRunner{}
	newMigrationSource = func() (source.Driver, error) {
		return fakeSourceDriver{}, nil
	}
	newMigrationRunner = func(src source.Driver, databaseURL string) (migrationRunner, error) {
		if src == nil {
			t.Fatal("newMigrationRunner received nil source")
		}
		if databaseURL != "postgres://mission:mission@localhost:5432/mission" {
			t.Fatalf("databaseURL = %q, want original URL", databaseURL)
		}
		return runner, nil
	}

	err := Migrate("postgres://mission:mission@localhost:5432/mission")
	if err != nil {
		t.Fatalf("Migrate returned error: %v", err)
	}
	if !runner.upCalled {
		t.Fatal("Up was not called")
	}
	if !runner.closeCalled {
		t.Fatal("Close was not called")
	}
}

func TestMigrateErrNoChange(t *testing.T) {
	restoreMigrateSeams(t)

	runner := &fakeMigrationRunner{upErr: migrate.ErrNoChange}
	newMigrationSource = func() (source.Driver, error) {
		return fakeSourceDriver{}, nil
	}
	newMigrationRunner = func(source.Driver, string) (migrationRunner, error) {
		return runner, nil
	}

	if err := Migrate("postgres://mission"); err != nil {
		t.Fatalf("Migrate returned error for ErrNoChange: %v", err)
	}
	if !runner.closeCalled {
		t.Fatal("Close was not called")
	}
}

func TestMigrateSourceError(t *testing.T) {
	restoreMigrateSeams(t)

	wantErr := errors.New("source failed")
	newMigrationSource = func() (source.Driver, error) {
		return nil, wantErr
	}

	err := Migrate("postgres://mission")
	if err == nil {
		t.Fatal("Migrate returned nil error")
	}
	if !strings.Contains(err.Error(), "load migration source") {
		t.Fatalf("error = %v, want wrapped load migration source error", err)
	}
}

func TestMigrateRunnerError(t *testing.T) {
	restoreMigrateSeams(t)

	wantErr := errors.New("runner failed")
	newMigrationSource = func() (source.Driver, error) {
		return fakeSourceDriver{}, nil
	}
	newMigrationRunner = func(source.Driver, string) (migrationRunner, error) {
		return nil, wantErr
	}

	err := Migrate("postgres://mission")
	if err == nil {
		t.Fatal("Migrate returned nil error")
	}
	if !strings.Contains(err.Error(), "init migrator") {
		t.Fatalf("error = %v, want wrapped init migrator error", err)
	}
}

func TestMigrateUpError(t *testing.T) {
	restoreMigrateSeams(t)

	runner := &fakeMigrationRunner{upErr: errors.New("apply failed")}
	newMigrationSource = func() (source.Driver, error) {
		return fakeSourceDriver{}, nil
	}
	newMigrationRunner = func(source.Driver, string) (migrationRunner, error) {
		return runner, nil
	}

	err := Migrate("postgres://mission")
	if err == nil {
		t.Fatal("Migrate returned nil error")
	}
	if !strings.Contains(err.Error(), "apply migrations") {
		t.Fatalf("error = %v, want wrapped apply migrations error", err)
	}
	if !runner.closeCalled {
		t.Fatal("Close was not called")
	}
}
