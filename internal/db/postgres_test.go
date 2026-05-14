package db_test

import (
	"Diaspora/internal/db"
	"testing"
)

func Setup() (*db.PostgresDB, error) {
	dsn := "host=localhost port=5432 user=AdminDias password=Admin dbname=diaspora_test sslmode=disable"
	postdb, err := db.NewPostgresDB(dsn)
	if err != nil {
		return nil, err
	}
	return postdb, nil
}

func TestAlive(t *testing.T) {
	postdb, err := Setup()
	if err != nil {
		t.Fatal(err)
	}
	if err := postdb.Alive(); err != nil {
		t.Fatal(err)
	}
}

func TestBeginTx(t *testing.T) {
	postdb, err := Setup()
	if err != nil {
		t.Fatal(err)
	}
	tx, err := postdb.BeginTx(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	tx.Exec(t.Context(), `CREATE TABLE IF NOT EXISTS test (id SERIAL PRIMARY KEY)`)
	if err := tx.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}
	tx, err = postdb.BeginTx(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	tx.Exec(t.Context(), `INSERT INTO test DEFAULT VALUES`)
	if err := tx.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}
	tx, err = postdb.BeginTx(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	var count int
	if err := tx.QueryRow(t.Context(), `SELECT COUNT(*) FROM test`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 row, got %d", count)
	}
}
