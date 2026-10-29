# Validation record — 2026-10-26

The local toolchain is Go 1.27.3 on Linux. The CLI now requires Go 1.25 for Charm v2; generated applications still declare Go 1.24. Compatibility with those minimum toolchains is assigned to the checked-in CI jobs and has not been executed locally.

## Interactive wizard update

- Added the actual Bubble Tea v2 wizard, Bubbles inputs/spinner/viewport, and Lip Gloss styling. `gbme` starts the wizard on a suitable terminal; explicit `new` calls use flags unless `--interactive` is requested. Non-terminal input/output, CI and dumb terminals receive an actionable error instead of prompting.
- Model tests drive navigation, text editing, selections, conditional SQL visibility, project/module persistence, compatibility errors, invalid/existing/symlink destinations, cancellation and cleanup waiting, failure/retry, duplicate-submission prevention, narrow layouts, plain output and next commands. The real generator is exercised across all 12 framework/database/optional-feature combinations; a stdlib wizard flow additionally verifies the generated module.
- `go test -short ./...`, `GBME_MATRIX=1 go test -race -count=1 ./...`, `go vet ./...` and `make build` passed during implementation using the offline cache described below. These do not establish Gin compilation or live database/Redis/Docker startup.
- A real pseudo-terminal smoke test launched bare `gbme`, entered a name, changed HTTP/database choices with arrow keys, enabled Redis/Docker with Space, reviewed the summary, generated, and exited. No project existed before confirmation. The generated manifest/files matched stdlib + PostgreSQL/sql + Redis + Docker, and persistent success output showed the correct absolute path, `.env` copy and Compose startup commands. Terminal modes were restored on exit.
- The separate Gin executable test was attempted and remains blocked by the missing cached Gin v1.11.0 dependency. Its templates and wizard integration render successfully; it remains labeled preview.
- CI now builds/tests the CLI on Go 1.25 and stable. A separate job builds the CLI on 1.25, switches to Go 1.24, and verifies generated stdlib/Gin/Fiber PostgreSQL/Redis/Docker apps and CRUD additions. This keeps generated-app minimum-version coverage independent of the TUI's requirements.

The original pre-TUI validation record follows; its coverage figures refer to that earlier implementation.

## Passed

- Generator CLI build and local installation: `go build -buildvcs=false -trimpath -o bin/gbme ./cmd/gbme`, `go install -buildvcs=false ./cmd/gbme`, installed `gbme version` prints `0.1.0`.
- `go vet ./...`.
- `GBME_MATRIX=1 go test -race -count=1 -cover ./...`: all generator packages passed. Coverage: CLI 87.5%, config 71.1%, generator 78.3%. Main executable wrapper has no direct unit test; CLI behavior and installation were exercised.
- Matrix renders/parses/formats 12 combinations across stdlib/Gin/Fiber, no database/Postgres, and selected optional features. Executable matrix resolves/tests/builds Fiber-only, Postgres-only, Redis-only, Fiber toolbox and stdlib toolbox projects, plus the default stdlib executable test. Resources/endpoints are then added and the staged projects tested/built again. GET-only bool fields, POST-only string fields, mixed numeric/bool/string fields and zero-value replacement are exercised.
- Generator tests cover CLI flags, defaults/wizard/EOF, invalid combinations, name/path checks, manifest ownership, framework output, conditional dependencies, required files, duplicate generation, modified registries, symlinks, project locks, migration sequencing, canceled work, failed verification cleanup and publication rollback preserving a raced-in user file.
- All three checked-in standalone examples were generated through the compiled CLI with `--verify`; each subsequently passed `go test -race -count=1 ./...`, `go vet ./...` and `go mod verify`. The generator module also passed `go mod verify`.
- Redis default tests use go-redis itself over an in-process RESP fixture with net.Pipe: ping, miss, set/get, example population, error propagation, invalid TTL and cancellation pass. No replacement/mock Redis client is substituted.
- PostgreSQL 18.6 standalone backend (no sockets) executed the generated example schema/up/down SQL, create/read/update/delete operations, zero/false updates and transaction rollback. This verifies SQL execution, not network pgx integration or migration version bookkeeping.
- `docker compose --env-file examples/toolbox/.env.example -f examples/toolbox/compose.yaml config --quiet` and the equivalent postgres-api command passed.
- SHA-256 comparisons confirm all 326 reference-repository files are unchanged. Every owned file in the refreshed examples matches its manifest hash (19 minimal, 32 PostgreSQL, 39 toolbox files).

## Environmental limits and failed probes

- DNS/network access is unavailable (`proxy.golang.org` could not resolve). Dependency resolution used the existing module download cache as a read-only file proxy. Compatible Go 1.24 transitive versions are pinned in templates. Cache writes were redirected to /tmp.
- Gin v1.11.0 is absent from that cache. `gbme new gin-validation --http gin --verify` failed dependency resolution and cleaned up its staged output. Gin source/handler/middleware tests are implemented and its templates render/parse/format, but executable validation is pending. It remains a preview.
- Docker CLI/Compose are installed; daemon access is denied. Images, container startup, dependency health and one-shot migrations were not executed.
- Local TCP and Unix sockets are forbidden. Initializing an isolated PostgreSQL cluster succeeded; starting its listener failed with operation not permitted. A generated native server likewise returned a clear bind error and exit status 1. In-process stdlib handlers and Fiber App.Test requests work, but live listener/graceful-drain integration was not exercised.
- No Redis server is installed, and sockets are unavailable. External PostgreSQL/Redis tests explicitly skip without TEST_DATABASE_URL/TEST_REDIS_URL. The RESP fixture passes; that is not proof against an actual Redis service.
- golang-migrate is not installed/cached. The pinned Make/Docker workflows are present, but their executable download/version tracking has not been verified here. SQL up/down files were separately executed with PostgreSQL standalone mode.
- The workspace's enclosing .git metadata is inaccessible to Go VCS stamping. Local builds/installations and generator validation use `-buildvcs=false`; this does not affect application functionality.

## Reproduce the offline checks in this sandbox

```sh
export GOPATH=/tmp/gbme-go
export GOMODCACHE=/tmp/gbme-modcache
export GOCACHE=/tmp/gbme-buildcache
export GOPROXY=file:///home/ruhin/go/pkg/mod/cache/download
export GOSUMDB=off
go mod tidy
go vet ./...
GBME_MATRIX=1 go test -race -count=1 -cover ./...
make build
go install -buildvcs=false ./cmd/gbme
```

GOSUMDB=off is only for this offline reproduction; normal online workflows should retain Go's default checksum verification. Local module archive integrity was checked by go mod verify.

The checked-in CI workflow includes Go 1.24/stable, Gin, actual PostgreSQL/Redis service tests and a separate Docker startup job. CI has not been run in this session. In a network/service-enabled environment, run `GBME_MATRIX=1 GBME_GIN_MATRIX=1 go test -count=1 ./...` with disposable TEST_DATABASE_URL/TEST_REDIS_URL, then build/start the toolbox with Compose and exercise CRUD, cache, readiness and shutdown before promoting previews or claiming deployment validation.
