package storage

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct {
	sql *sql.DB
}

func Open(path string) (*DB, error) {
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	raw.SetMaxOpenConns(1)
	raw.SetConnMaxLifetime(time.Hour)

	db := &DB{sql: raw}
	if err := db.Migrate(context.Background()); err != nil {
		raw.Close()
		return nil, err
	}
	return db, nil
}

func (db *DB) Close() error {
	return db.sql.Close()
}

func (db *DB) SQL() *sql.DB {
	return db.sql
}

func (db *DB) HasTableForTest(t *testing.T, name string) bool {
	t.Helper()
	var count int
	err := db.sql.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&count)
	if err != nil {
		t.Fatalf("checking table %s: %v", name, err)
	}
	return count == 1
}
