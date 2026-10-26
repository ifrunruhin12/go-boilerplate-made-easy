# Standalone generated examples

| Directory | Composition | Connected examples |
|---|---|---|
| minimal | stdlib, no database/Redis/Docker | GET/POST users |
| postgres-api | stdlib, PostgreSQL/sql, Docker | Persistent products CRUD |
| toolbox | Fiber, PostgreSQL/sql, Redis, Docker | Products CRUD and in-memory users |

Each directory has its own Go module, generated manifest, README, configuration and tests. They were generated through the compiled CLI with `--verify`. No example imports the generator. Gin remains a template preview pending dependency access; use `gbme new gin-api --http gin --verify` to validate it in your environment.

```sh
cd minimal
go test ./...
go run ./cmd/server
```

Database/Redis examples require the documented exported environment values and services for native startup. For Compose, copy `.env.example` to `.env`, then run `docker compose up --build`. Tests run without services; integration tests explicitly skip until TEST_DATABASE_URL/TEST_REDIS_URL are set.

Recreate examples into a fresh temporary directory using:

```sh
gbme new minimal --dir /tmp --non-interactive --verify
gbme add endpoint users --project /tmp/minimal --fields name:string,email:string --verify
gbme new postgres-api --dir /tmp --http stdlib --database postgres --docker --verify
gbme add resource products --project /tmp/postgres-api --fields name:string,price:float64,active:bool --crud --verify
gbme new toolbox --dir /tmp --preset full --verify
gbme add resource products --project /tmp/toolbox --fields name:string,price:float64,quantity:int64,active:bool --crud --verify
gbme add endpoint users --project /tmp/toolbox --fields name:string,email:string --verify
```

Choose an unused parent/name when recreating; gbme will refuse existing destinations.
