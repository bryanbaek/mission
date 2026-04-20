package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/bryanbaek/mission/internal/controlplane/db"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: preflight-helper migrations-status --database-url=<url>")
	}

	switch args[0] {
	case "migrations-status":
		return runMigrationsStatus(args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runMigrationsStatus(args []string) error {
	fs := flag.NewFlagSet("migrations-status", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var databaseURL string
	fs.StringVar(&databaseURL, "database-url", "", "Postgres database URL")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if databaseURL == "" {
		return fmt.Errorf("--database-url is required")
	}

	status, err := db.MigrationState(databaseURL)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(status)
}
