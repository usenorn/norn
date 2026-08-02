package db

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed migrations/postgres/*.sql
var migrations embed.FS

func PostgresMigrations() (fs.FS, error) {
	fsys, err := fs.Sub(migrations, "migrations/postgres")
	if err != nil {
		return nil, fmt.Errorf("open postgres migrations: %w", err)
	}

	return fsys, nil
}
