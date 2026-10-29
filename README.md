# Go Boilerplate Made Easy

**`gbme` composes runnable Go backends and adds connected endpoints and PostgreSQL CRUD resources.** Applications are ordinary Go modules with no runtime dependency on the generator. Choose infrastructure at generation time; disabled features add no initialization or direct dependency.

```sh
go build -buildvcs=false -o bin/gbme ./cmd/gbme
./bin/gbme new my-api --non-interactive --verify
cd my-api
go run ./cmd/server
```

Go **1.25+** is required to build/install GBME; generated applications still require **Go 1.24+**. The CLI uses Charm's maintained v2 libraries (Bubble Tea 2.0.8, Bubbles 2.1.1, Lip Gloss 2.0.6), which require Go 1.25. The existing local directory is `go-boilerplate-made-easy`; the generator module identifier preserves the requested `go-boiletplate-made-easy` spelling. No public installation URL is assumed.

## Install and quick start

From this directory:

```sh
go mod download
go install -buildvcs=false ./cmd/gbme       # put $(go env GOPATH)/bin on PATH
export PATH="$(go env GOPATH)/bin:$PATH"   # persist this in ~/.zshrc or ~/.bashrc
gbme version
gbme                       # launch the interactive project wizard
# In CI/pipes, defaults are applied without prompts:
gbme new another-api --non-interactive --verify
```

`--verify` resolves dependencies, runs generated tests and builds in a staging directory before publishing. It needs access to the selected dependencies, but ordinary unit tests need no running services. Without it, all Go source is formatted and configuration is validated; run `go mod tidy` in the result before building. Existing destinations are always refused. `--dir` selects an existing parent directory; names are lowercase kebab-case. Use `--module example.com/team/my-api` for an import path.

## Interactive wizard

Run `gbme` in a terminal. The wizard asks for a project name and existing parent directory, HTTP implementation, database, optional Redis and Docker, and whether to verify the generated project. PostgreSQL adds a SQL data-access question; choosing no database skips it. A final review shows every selection and the resolved output path before Enter starts generation.

| Key | Action |
|---|---|
| Arrow keys | Select an option |
| Space | Toggle Redis, Docker or verification |
| Enter | Continue; generate on the review screen; exit on the result screen |
| Esc | Previous question; return from an error to review |
| Backspace | Previous question on selection screens; delete text in inputs |
| Up/Down | Scroll review, results and diagnostics |
| Page Up/Page Down | Scroll any question or result on a small terminal |
| Ctrl+C | Cancel; during generation wait for generator cleanup |

Selections persist when navigating back. Names must be lowercase kebab-case with at most 64 characters. Paths may be absolute, relative or start with `~/`; the parent must already exist and the target must be new. Redis adds the implemented client, readiness check and cache example; connection settings belong in the generated `.env`. Docker includes only selected services. Authentication, OpenAPI, MySQL, SQLite, GORM and sqlc are not selectable because they are not implemented. Gin is labeled as a preview.

Generation runs without blocking keyboard input and shows activity without estimated percentages. Verification can resolve dependencies, test and build before publishing; it has a five-minute timeout. Failures retain compiler/tool diagnostics and allow returning to review. The success screen prints the output path and native or Docker commands, including required environment loading and migrations. Those instructions remain in the terminal after exiting.

Seed the wizard with flags when useful:

```sh
gbme new my-api --interactive --http fiber --database postgres --redis --docker
gbme new my-api --interactive --module example.com/team/my-api --dir ~/Projects
```

**CLI compatibility:** explicit `gbme new NAME` invocations now use defaults/flags without automatic prompts. Use bare `gbme` or `new NAME --interactive` for the TUI. All existing generation flags, presets, `add`, `doctor`, `list`, `version` and help remain available. `--interactive` requires terminal input and output and cannot be combined with `--non-interactive`.

In CI or with redirected streams, bare `gbme` exits with guidance instead of waiting for input. Supply `gbme new NAME --non-interactive [flags]` for scripts:

```sh
gbme new my-api --http gin --database postgres --docker --non-interactive
gbme --help
gbme version
```

For a local binary without installing, run `make build` and `./bin/gbme`. Run `make install` again after updating GBME to refresh the installed command.

```sh
gbme new billing-api --http stdlib --database postgres --data-access sql --redis --docker --verify
cd billing-api
cp .env.example .env
docker compose up --build
# A second terminal:
gbme add resource products --fields name:string,price:float64,active:bool --crud --verify
docker compose build
docker compose run --rm migrate
docker compose up --build
```

## Release scope and compatibility

| Choice | Implementation | Executable validation in this environment |
|---|---|---|
| stdlib | `net/http`, method/path routes and middleware | Passed |
| Fiber v2 | Native app/groups/context/handlers and shutdown | Passed |
| Gin | Native engine/groups/context/handlers and shutdown | Template parsing/formatting passed; compilation pending uncached dependency access |
| none / none | HTTP-only in-memory GET/POST endpoints | Passed |
| PostgreSQL / sql | pgx stdlib pool, parameterized typed repositories, transactions, migrations | Builds/unit tests passed; SQL exercised in standalone PostgreSQL; network integration blocked |
| Redis | Optional go-redis client, readiness and cache read/write example | Passed with real client over RESP/net.Pipe fixture; external-service test pending |
| Docker | Optional multi-stage, non-root image, selective Compose, health checks, migration service | Compose config validated; build/start pending Docker daemon/network access |

All three HTTP templates compose with PostgreSQL/sql, Redis and Docker. Gin is a **preview until its executable matrix passes**. GORM, sqlc, SQLite, MySQL and MongoDB are deliberately rejected with helpful errors. They are planned extensions, not advertised working integrations. The current release prioritizes complete PostgreSQL/sql and HTTP-only workflows over a large dependency matrix. See [research and decisions](docs/research.md) and [validation record](docs/validation.md).

Core capabilities: typed environment configuration, slog JSON logging, strict request validation, response/error envelopes, request IDs, configurable timeouts/CORS, liveness/readiness, explicit dependency construction and graceful shutdown. Optional integrations: Redis and Docker. Authentication/JWT, authorization, OpenAPI, rate limiting, seeding, uploads, email, jobs, brokers, object storage and telemetry are not implemented. Public example endpoints need application-specific authorization before handling sensitive data.

## CLI reference

| Command | Purpose |
|---|---|
| `gbme` | Launch the interactive project-generation wizard in a terminal |
| `gbme new NAME` | Generate a separate Go module |
| `gbme add endpoint NAME --fields ...` | In-memory HTTP example with GET and/or POST |
| `gbme add resource NAME --fields ... --crud` | PostgreSQL-backed GET, POST, PUT and DELETE |
| `gbme list [frameworks\|databases\|features\|presets]` | Choices and validation status |
| `gbme doctor [--project PATH] [--verify]` | Locate tools, inspect manifest, optionally tidy/test/build |
| `gbme version` | Generator version |

Every command has `--help`. `new` accepts `--http`, `--database`, `--data-access`, `--redis`, `--docker`, `--module`, `--dir`, `--preset`, `--interactive`, `--non-interactive` and `--verify`. `--orm` is a compatibility alias for `--data-access`; `--orm gorm` explains that GORM is not implemented. Explicit commands do not launch the wizard unless `--interactive` is provided. `--preset full` selects Fiber + PostgreSQL/sql + Redis + Docker; individual flags override preset defaults, including `--redis=false` and `--docker=false`.

`add` accepts `--project PATH` (default current directory), `--fields` and `--verify`; endpoint additionally accepts `--method GET,POST`; resource requires `--crud`. There is no duplicate `generate` alias and no automatic `remove`: safe destructive pruning is deferred.

## Endpoint and resource generation

```sh
gbme add endpoint users --method GET,POST --fields name:string,email:string --verify
gbme add resource products --fields name:string,price:float64,quantity:int64,active:bool --crud --verify
```

The second command requires a PostgreSQL project. Resource names are lowercase Go identifiers without underscores. Fields support `string`, `int`, `int64`, `float64` and `bool`; snake_case field names map to exported Go fields and JSON keys. Duplicate/reserved names and unsupported types are rejected. All input fields must be present; zero and false are valid. Strings are required, capped at 255 characters, and `email` fields receive email validation. Bodies must be single JSON objects with no unknown fields and at most 1 MiB. PUT fully replaces fields; PATCH and custom path/query specifications are deferred.

Generated routes are registered at `/api/v1/NAME`. GET lists accept `limit` (default 20, max 100) and `offset`; persistent resources also have `GET/PUT/DELETE /api/v1/NAME/{id}`. Responses use `{ "data": ... }` or `{ "error": { "code": ..., "message": ... } }`, with 201 for creates, 204 for deletes, 400 for invalid input and 404 for absent records. PostgreSQL queries bind all values and quote generated identifiers. In-memory examples reset on restart and cap at 10,000 records.

Each addition creates DTO/model, service/storage seam, framework-specific handler, unit tests and route registration; database resources additionally get a repository, service-gated integration test and up/down migrations. New migration versions account for existing manual migrations. Apply migrations and restart the server after adding resources. Existing domain code is preserved; only an unmodified generated route registry is replaced. A project lock protects concurrent gbme additions; failed verification publishes nothing. Ordinary publication failures roll back created files/registry. Multi-file additions are not crash-atomic; use version control and avoid editing the registry during generation.

## Generated architecture and configuration

```text
cmd/server/                dependency construction and lifecycle
internal/config/           typed env validation
internal/api/              envelopes, JSON parsing, pagination, IDs
internal/server/           chosen framework and generated registry
internal/database/         PostgreSQL/sql client and transaction example (optional)
internal/cache/            Redis client and cache example (optional)
internal/<resource>/       DTO, service, handler, tests, optional repository
migrations/                numbered SQL up/down pairs (PostgreSQL)
.gbme.json                 selections, resources, ownership and feature dependencies
```

The generator separates Cobra command handling (`internal/cli`), capability choices and validation (`internal/config`), Bubble Tea state/navigation/views (`internal/wizard`), and rendering/composition/file publication (`internal/generator`). The wizard submits configuration through a small `Generator` interface implemented by the existing `Engine.New`; it does not render templates or publish files. Templates are embedded in the binary; framework implementations are separate files. Generated applications have no dependency on the TUI. Normal Go interfaces isolate storage; the application entry point supplies dependencies explicitly.

Generated READMEs contain the complete env reference and workflows. Common variables are `HOST`, `PORT`, `READ_TIMEOUT`, `WRITE_TIMEOUT`, `IDLE_TIMEOUT`, `SHUTDOWN_TIMEOUT` and exact `CORS_ORIGINS`. PostgreSQL requires `DATABASE_URL` and supports `DB_MAX_OPEN`/`DB_MAX_IDLE`; Redis uses `REDIS_URL`, including `rediss://`. `.env.example` contains local-only example values. Native applications load exported variables: `set -a; . ./.env; set +a`. They do not implicitly read dotenv. Deploy secrets through your environment/secret manager and enable TLS in production.

## Migrations, Docker and pruning

PostgreSQL projects use pinned **golang-migrate v4.19.0** through Make's `go run` workflow; an arbitrary host installation is unnecessary. `make migrate-up`, `make migrate-down` (one version), `make migrate-status` and `NAME=add_index make migrate-create` are documented in generated projects. Review editable migrations before applying. Startup never silently changes schemas. The reproducible tool download and Docker build require network access.

Docker is selectable. Compose starts only selected PostgreSQL/Redis services, persists their state, waits for health, and runs PostgreSQL migrations before HTTP. The image uses a multi-stage static build and a non-root runtime with CA certificates. Native service ports and application publishing are loopback-bound in the local Compose setup. `docker compose down` retains volumes. Native workflows are available regardless of Docker selection.

`gbme new reference --preset full` produces a curated toolbox. `.gbme.json` records direct Go dependencies, feature requirements, resource specifications, owned files/hashes and shared integration files. For AI-assisted pruning, use these integration points to remove clients, routes, config and dependencies together, then tidy/test/build. There is no promise of automated safe removal; modified files must be reviewed.

## Testing and examples

```sh
go test ./...                        # generator, CLI, validation and executable stdlib tests
GBME_MATRIX=1 go test -count=1 ./...   # stdlib/Fiber, Postgres, Redis, Docker combinations
GBME_MATRIX=1 GBME_GIN_MATRIX=1 go test -count=1 ./... # include Gin; needs dependencies
go test -race ./...
go vet ./...
```

Set `TEST_DATABASE_URL` and `TEST_REDIS_URL` to disposable services to run generated integration tests inside the matrix. PostgreSQL tests use session-local temporary tables; Redis tests use unique keys and clean them up. Without service URLs those tests explicitly skip; the Redis protocol fixture runs in ordinary tests. `--short` skips executable generated-project checks. CI specifies PostgreSQL/Redis services and includes the Gin matrix; its execution has not been observed locally.

[Examples](examples/README.md) include a minimal stdlib API, a stdlib PostgreSQL resource project, and a Fiber toolbox project. Each is a standalone module with its own README and tests. Generator tests cover CLI parsing, incompatible options, rendering, framework/database output, required files, naming/paths, selected dependencies, duplicates, locks, symlinks, modified registries, migration sequencing and failure cleanup. Representative combinations are executed, not merely snapshotted.

Wizard tests drive Bubble Tea messages without a real terminal: forward/back navigation, text editing, retained selections, conditional questions, validation, cancellation/cleanup, errors/retry, final submission, narrow/plain rendering, startup commands and integration with the real generator. A verified stdlib wizard flow executes generated tests and a build unless `-short` is supplied.

## Troubleshooting and contribution

Terminal problems: the wizard requires both stdin and stdout to be terminals, a nonempty `TERM` other than `dumb`, and an unset `CI` variable. Use a normal terminal or the explicit non-interactive command if an IDE console, pipe or remote session lacks terminal support. `NO_COLOR=1 gbme` disables styling; selections also use text markers. Narrow layouts wrap, and review/results/errors can scroll. The wizard restores the terminal on exit. If `gbme` still shows old behavior after rebuilding, reinstall with `make install` and check `command -v gbme`.

Missing dependencies: run `go mod tidy`, or use `--verify` where your proxy is available. No network: stdlib generation/tests remain usable; cached modules can be used through a file GOPROXY. A failed verification removes the staged project. Tool errors identify the failing Go command.

Database/Redis readiness failures: check exported URLs and service reachability. Bind failures are fatal and return nonzero. Docker permission errors require a usable Docker daemon; `doctor` checks tool presence, not daemon authorization. A modified `resources_gen.go` blocks additions: move custom registrations into `server.go` and restore the tracked generated registry. Avoid deleting `.gbme.lock` while a generator is active.

See [CONTRIBUTING.md](CONTRIBUTING.md), [LICENSE](LICENSE) and [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md). Both research references are MIT-licensed; their useful ideas were independently implemented, and no source/template blocks were copied.
