# Contributing

Build the CLI with Go 1.25+ (Charm v2's minimum); generated modules remain Go 1.24+. Run `go mod tidy`, `go test ./...` and `go vet ./...`. Use `GBME_MATRIX=1 GBME_GIN_MATRIX=1 go test -count=1 ./...` with dependency access before claiming full framework support. Set `TEST_DATABASE_URL`/`TEST_REDIS_URL` to disposable services for integration checks. Format Go with gofmt. Generated modules have no runtime dependency on gbme.

Keep CLI handling in internal/cli, choice validation in internal/config, and composition/file ownership in internal/generator. Put materially different framework implementations in their own embedded template files. Add a feature only with a working integration, clear compatibility rules, conditional dependencies, generated tests, documentation and an executable matrix case. Prefer storage interfaces scoped to a domain and explicit dependency construction over global clients or a universal database layer.

Keep terminal state, navigation and views in internal/wizard. Reuse the config capability catalog and submit through the Generator interface; Bubble Tea Update handlers must not render templates or write project files. Test transitions with messages and generation with the real Engine as well as controlled error/cancellation implementations. Keep explicit CLI commands usable with redirected streams and in CI. During cancellation, wait for the generator to return before exiting the UI.

When changing templates, generate into fresh directories; do not overwrite user projects or reference repositories. Regenerate examples into a temporary parent and review diffs before replacing checked-in examples. Include `--verify`, handler behavior, failure cleanup and manifest ownership checks. Run Docker Compose config checks and, when available, actual builds/services/migrations. Report unavailable services honestly.

Highest-value next work: execute the Gin/maintained-migration/Docker matrix in an unrestricted environment; add sqlc with pinned regeneration and committed queries; add pure-Go SQLite with migrations; generate OpenAPI from resources; add tested optional authentication and rate limiting. Automated removal must detect modifications and validate feature dependencies before any destructive operation.

Submit focused changes describing observable behavior and validation. Do not publish or commit on a user's behalf without authorization. All contributions must be compatible with MIT licensing; retain notices for any reused source.
