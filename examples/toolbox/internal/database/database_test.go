package database

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"
	"toolbox/internal/config"
)

func TestRejectInvalidURL(t *testing.T) {
	if db, err := Open(context.Background(), config.Config{DatabaseURL: "://invalid"}); err == nil {
		db.Close()
		t.Fatal("invalid URL accepted")
	}
}
func TestTransactionsIntegration(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to a disposable PostgreSQL instance")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := Open(ctx, config.Config{DatabaseURL: url, DBMaxOpen: 1, DBMaxIdle: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.ExecContext(ctx, "CREATE TEMP TABLE notes (id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY, body TEXT NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	id, err := AddNote(ctx, db, "hello")
	if err != nil || id < 1 {
		t.Fatalf("%d %v", id, err)
	}
	sentinel := errors.New("rollback")
	err = Transaction(ctx, db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "INSERT INTO notes (body) VALUES ($1)", "rollback"); err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
	var count int
	if err = db.QueryRowContext(ctx, "SELECT count(*) FROM notes").Scan(&count); err != nil || count != 1 {
		t.Fatalf("rollback failed: %d %v", count, err)
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Error("panic not propagated")
			}
		}()
		_ = Transaction(ctx, db, func(tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, "INSERT INTO notes (body) VALUES ($1)", "panic")
			if err != nil {
				t.Fatal(err)
			}
			panic("example")
		})
	}()
	if err = db.QueryRowContext(ctx, "SELECT count(*) FROM notes").Scan(&count); err != nil || count != 1 {
		t.Fatalf("panic rollback failed: %d %v", count, err)
	}
}
