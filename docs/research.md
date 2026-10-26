# Source reconnaissance and first-release plan

The local destination is `go-boilerplate-made-easy`; the requested module identity is `go-boiletplate-made-easy`. Both local references were inspected before implementation, including actual server, database, middleware, authentication, domain, configuration, Docker, migration and test sources. Both are MIT licensed. This implementation adapts architectural ideas independently, without copying source or templates.

| Area | vahiiiid / GRAB | Codoworks |
|---|---|---|
| Layout | cmd entry points, internal domain handler/service/repository | application CLI, pkg clients, feature registry, models, routers, proc |
| Go and dependencies | Go 1.24, Gin/GORM/Postgres, Viper, JWT v5, migrate, Swagger | Go 1.21, Echo/GORM, Cobra/Viper, Logrus, Kratos/Keto, gormigrate |
| HTTP | grouped routes, envelope errors, logger, rate limits | three listeners and route classes, ordered middleware |
| Persistence | repository interfaces, context transactions, SQL migrations, SQLite tests | adapter registry for Postgres/MySQL/SQLite, model persistence, Go migration and seed registry |
| Lifecycle | SIGINT/SIGTERM; closes DB before draining server: redesign | handles interrupt; global clients/config; fatal exits: redesign |
| Configuration | typed YAML/env, validation, safe logging; duplicate DB constructors | reflection-based features and environment printing; avoid complexity and secret exposure |
| Security | bcrypt, JWT refresh rotation and RBAC; useful future reference | external Kratos sessions; authorization source empty; avoid global request session state |
| Redis | no integration found | RedisClient.Init empty: do not treat advertised toggle as working Redis |
| Tests | substantial domain, handler, migration, config and health tests | no Go test files found |
| Operations | Docker-first Makefile, migration CLI, hot reload, Swagger generation | nested Compose extends, tasks/watcher/seed commands, scratch image |
| Limits | fixed Gin/Postgres stack, many mandatory features/dependencies; permissive CORS | generator absent; feature disabling keeps dependencies; global models/clients reduce testability |

Adapt explicit construction, domain boundaries, readiness checks, migration workflows and practical operational commands. Redesign feature selection as generation-time composition, independent framework templates, atomic filesystem publication and manifest ownership. Discard mandatory auth, three-listener defaults, runtime reflection registries, body/session dumps and overlapping logging/config libraries.

Implementation sequence: (1) Cobra/config/embedded rendering and stdlib vertical slice; (2) PostgreSQL/sql, Redis, Docker, Gin and Fiber templates; (3) typed resource service, framework handlers, parameterized SQL, route regeneration; (4) executable generated-project matrix, examples, docs and failure-path checks. Minimum Go 1.24. No runtime dependency on gbme. PostgreSQL/sql is the initial database pair; GORM/sqlc/SQLite/MySQL/MongoDB are deliberately deferred. sqlc should eventually use a pinned tool workflow plus committed generated queries, not require an arbitrary installed version.

Core: typed env config, slog, strict JSON validation, envelopes, request IDs, CORS, lifecycle, health/readiness, tests. Optional: Redis and Docker. Future priorities: auth with explicit threat model, OpenAPI from the resource manifest, sqlc, SQLite, metrics/tracing, rate limits. Uploads, email, jobs, brokers and object storage are deferred to avoid unfinished integrations.

Environment: Go 1.27.3 is installed. Network DNS is unavailable; Docker daemon access is denied. Cached Cobra, Fiber, pgx and Redis allow offline executable validation. Gin and pinned golang-migrate workflows require dependency/network access; these limits must remain explicit in the validation report.
