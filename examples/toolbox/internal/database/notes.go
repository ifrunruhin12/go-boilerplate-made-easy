package database

import (
	"context"
	"database/sql"
	"fmt"
)

// AddNote demonstrates parameterized SQL inside an explicitly scoped transaction.
func AddNote(ctx context.Context, db *sql.DB, body string) (int64, error) {
	if body == "" {
		return 0, fmt.Errorf("note body is required")
	}
	var id int64
	err := Transaction(ctx, db, func(tx *sql.Tx) error {
		return tx.QueryRowContext(ctx, "INSERT INTO notes (body) VALUES ($1) RETURNING id", body).Scan(&id)
	})
	return id, err
}
