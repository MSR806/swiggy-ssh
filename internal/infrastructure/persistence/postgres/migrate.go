package postgres

import (
	"context"
	"database/sql"
	"embed"
	"errors"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/lib/pq"
)

//go:embed migrations/*.sql
var embeddedMigrations embed.FS

func MigrateUp(_ context.Context, databaseURL string) error {
	m, err := newMigrator(databaseURL)
	if err != nil {
		return err
	}
	defer func() { _, _ = m.Close() }()

	err = m.Up()
	if err == nil || errors.Is(err, migrate.ErrNoChange) {
		return nil
	}

	return err
}

func MigrateDown(_ context.Context, databaseURL string) error {
	m, err := newMigrator(databaseURL)
	if err != nil {
		return err
	}
	defer func() { _, _ = m.Close() }()

	err = m.Steps(-1)
	if err == nil || errors.Is(err, migrate.ErrNoChange) {
		return nil
	}

	return err
}

func MigrateDrop(_ context.Context, databaseURL string) error {
	m, err := newMigrator(databaseURL)
	if err != nil {
		return err
	}
	defer func() { _, _ = m.Close() }()

	err = m.Drop()
	if err == nil || errors.Is(err, migrate.ErrNoChange) {
		return nil
	}

	return err
}

func newMigrator(databaseURL string) (*migrate.Migrate, error) {
	sourceDriver, err := iofs.New(embeddedMigrations, "migrations")
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, err
	}

	dbDriver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	return migrate.NewWithInstance("iofs", sourceDriver, "postgres", dbDriver)
}
