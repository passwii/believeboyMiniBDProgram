package database

import (
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	dbfiles "github.com/believeboy/mini-bd-program/services/api/db"
)

func Migrate(databaseURL, command string) error {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return fmt.Errorf("open migration database: %w", err)
	}
	defer db.Close()

	goose.SetBaseFS(dbfiles.Migrations)
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}
	if err := goose.Run(command, db, "migrations"); err != nil {
		return fmt.Errorf("run migration %s: %w", command, err)
	}
	return nil
}
