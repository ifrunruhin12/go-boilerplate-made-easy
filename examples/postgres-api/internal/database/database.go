package database

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"postgres-api/internal/config"
	"time"
)

func Open(ctx context.Context, c config.Config) (*sql.DB, error) {
	connection, err := pgx.ParseConfig(c.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid DATABASE_URL; check the PostgreSQL connection settings")
	}
	connection.ConnectTimeout = 5 * time.Second
	db := stdlib.OpenDB(*connection)
	db.SetMaxOpenConns(c.DBMaxOpen)
	db.SetMaxIdleConns(c.DBMaxIdle)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("PostgreSQL connection failed; check URL, TLS and service reachability")
	}
	return db, nil
}
func Transaction(ctx context.Context, db *sql.DB, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
