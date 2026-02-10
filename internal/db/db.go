package db

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
)

const (
	driver = "sqlite"
	schema = `CREATE TABLE IF NOT EXISTS "scheduler" (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    date CHAR(8) NOT NULL DEFAULT "",
    title VARCHAR(64) NOT NULL DEFAULT "",
    comment TEXT NOT NULL DEFAULT "",
    repeat VARCHAR(128) NOT NULL DEFAULT ""
);
CREATE INDEX scheduler_date ON scheduler(date);
`
)

var db *sql.DB

// Init initializes the database connection with the given file.
// If the database file doesn't exist, it will be created and the database schema will be applied.
// If the database file already exists, Init will only check if the database is accessible.
// If any error occurs during the initialization process, Init will return an error.
func Init(dbFile string) error {
	_, err := os.Stat(dbFile)
	needCreateDB := errors.Is(err, os.ErrNotExist)
	if err != nil && !needCreateDB {
		return fmt.Errorf("error checking database file '%s': %w", dbFile, err)
	}

	var openErr error
	db, openErr = sql.Open(driver, dbFile)
	if openErr != nil {
		return fmt.Errorf("error opening database '%s': %w", dbFile, openErr)
	}

	success := false
	defer func() {
		if !success {
			db.Close()
		}
	}()

	if needCreateDB {
		_, err = db.Exec(schema)
		if err != nil {
			return fmt.Errorf("error applying database schema '%s': %w", dbFile, err)
		}
		return nil
	}

	if err := db.Ping(); err != nil {
		return fmt.Errorf("error accessing database '%s': %w", dbFile, err)
	}

	success = true
	return nil
}

// Close closes the database connection.
// If any error occurs during the closing process, Close will return an error.
func Close() error {
	if err := db.Close(); err != nil {
		return fmt.Errorf("error closing database: %w", err)
	}
	return nil
}
