package products

import (
	"context"
	"database/sql"
	"errors"
	"postgres-api/internal/api"
)

type Repository struct{ db *sql.DB }

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

const columns = `"id", "name", "price", "active"`

func scan(row interface{ Scan(...any) error }) (Record, error) {
	var v Record
	err := row.Scan(&v.ID, &v.Name, &v.Price, &v.Active)
	if errors.Is(err, sql.ErrNoRows) {
		err = api.ErrNotFound
	}
	return v, err
}
func (r *Repository) List(ctx context.Context, limit, offset int) ([]Record, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+columns+` FROM "products" ORDER BY "id" LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Record{}
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (r *Repository) Get(ctx context.Context, id int64) (Record, error) {
	return scan(r.db.QueryRowContext(ctx, `SELECT `+columns+` FROM "products" WHERE "id"=$1`, id))
}
func (r *Repository) Create(ctx context.Context, v Record) (Record, error) {
	return scan(r.db.QueryRowContext(ctx, `INSERT INTO "products" ("name", "price", "active") VALUES ($1, $2, $3) RETURNING `+columns, v.Name, v.Price, v.Active))
}
func (r *Repository) Update(ctx context.Context, id int64, v Record) (Record, error) {
	return scan(r.db.QueryRowContext(ctx, `UPDATE "products" SET "name"=$2, "price"=$3, "active"=$4 WHERE "id"=$1 RETURNING `+columns, id, v.Name, v.Price, v.Active))
}
func (r *Repository) Delete(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM "products" WHERE "id"=$1`, id)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return api.ErrNotFound
	}
	return nil
}
