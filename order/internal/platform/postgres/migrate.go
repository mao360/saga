package postgres

import (
	"errors"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
)

func RunMigrations(databaseDSN, migrationsDir string) error {
	absPath, err := filepath.Abs(migrationsDir)
	if err != nil {
		return err
	}

	m, err := migrate.New("file://"+filepath.ToSlash(absPath), databaseDSN)
	if err != nil {
		return err
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}

	return nil
}
