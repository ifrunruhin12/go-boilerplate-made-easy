.PHONY: run test build migrate-up migrate-down migrate-status migrate-create
run:
	go run ./cmd/server
test:
	go test ./...
build:
	go build -buildvcs=false -o bin/server ./cmd/server

# Export DATABASE_URL before running. This pinned tool is independent of application dependencies.
MIGRATE = go run -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@v4.19.0
migrate-up:
	$(MIGRATE) -path migrations -database "$${DATABASE_URL:?export DATABASE_URL first}" up
migrate-down:
	$(MIGRATE) -path migrations -database "$${DATABASE_URL:?export DATABASE_URL first}" down 1
migrate-status:
	$(MIGRATE) -path migrations -database "$${DATABASE_URL:?export DATABASE_URL first}" version
migrate-create:
	$(MIGRATE) create -ext sql -dir migrations -seq "$${NAME:?set NAME to a descriptive migration name}"
