package products

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
	"toolbox/internal/api"
	"toolbox/internal/config"
	"toolbox/internal/database"
)

func TestRepositoryIntegration(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL for PostgreSQL repository tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := database.Open(ctx, config.Config{DatabaseURL: url, DBMaxOpen: 1, DBMaxIdle: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// Session-local temporary tables keep tests isolated from application state.
	if _, err = db.ExecContext(ctx, `CREATE TEMP TABLE "products" (id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY, "name" TEXT NOT NULL, "price" DOUBLE PRECISION NOT NULL, "quantity" BIGINT NOT NULL, "active" BOOLEAN NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	r := NewRepository(db)
	v, err := r.Create(ctx, sampleInput().Record())
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.Get(ctx, v.ID)
	if err != nil || got != v {
		t.Fatalf("read: %v %v", got, err)
	}
	rows, err := r.List(ctx, 20, 0)
	if err != nil || len(rows) != 1 {
		t.Fatalf("list: %v %v", rows, err)
	}
	rows, err = r.List(ctx, 1, 1)
	if err != nil || len(rows) != 0 {
		t.Fatalf("pagination: %v %v", rows, err)
	}
	got, err = r.Update(ctx, v.ID, zeroInput().Record())
	expected := zeroInput().Record()
	expected.ID = v.ID
	if err != nil || got != expected {
		t.Fatalf("update: %v %v", got, err)
	}
	if err = r.Delete(ctx, v.ID); err != nil {
		t.Fatal(err)
	}
	if err = r.Delete(ctx, v.ID); !errors.Is(err, api.ErrNotFound) {
		t.Fatal(err)
	}
	if _, err = r.Update(ctx, v.ID, expected); !errors.Is(err, api.ErrNotFound) {
		t.Fatal(err)
	}
}
