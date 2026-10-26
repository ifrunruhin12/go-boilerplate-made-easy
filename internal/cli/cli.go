package cli

import (
	"context"
	"fmt"
	"github.com/spf13/cobra"
	"go-boiletplate-made-easy/internal/config"
	"go-boiletplate-made-easy/internal/generator"
	"go-boiletplate-made-easy/internal/wizard"
	"io"
	"os/exec"
	"strings"
	"time"
)

func New(version string, in io.Reader, out io.Writer) *cobra.Command {
	root := &cobra.Command{Use: "gbme", Short: "Go Boilerplate Made Easy: compose a runnable Go backend", SilenceUsage: true, SilenceErrors: true, Args: cobra.NoArgs}
	root.Long = "Generate runnable Go backends. Run gbme without arguments in a terminal\nto launch the project wizard. Use explicit commands and flags for scripts."
	root.Example = "  gbme\n  gbme new my-api --http gin --database postgres --docker\n  gbme new my-api --interactive\n  gbme new my-api --non-interactive --verify"
	root.SetIn(in)
	root.SetOut(out)
	root.SetErr(out)
	engine := generator.Engine{Version: version}
	root.RunE = func(cmd *cobra.Command, args []string) error {
		return wizard.Run(cmd.Context(), in, out, wizard.State{Parent: "."}, engine)
	}
	var p config.Project
	var parent, preset, orm string
	var interactive, nonInteractive, verify bool
	newCmd := &cobra.Command{Use: "new NAME", Short: "Generate a project without overwriting existing files", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		p.Name = args[0]
		if interactive && nonInteractive {
			return fmt.Errorf("--interactive and --non-interactive cannot be combined")
		}
		if preset != "" && preset != "full" {
			return fmt.Errorf("unknown preset %q; choose full", preset)
		}
		if preset == "full" {
			if !cmd.Flags().Changed("http") {
				p.HTTP = "fiber"
			}
			if !cmd.Flags().Changed("database") {
				p.Database = "postgres"
			}
			if !cmd.Flags().Changed("redis") {
				p.Redis = true
			}
			if !cmd.Flags().Changed("docker") {
				p.Docker = true
			}
		}
		if cmd.Flags().Changed("orm") {
			if cmd.Flags().Changed("data-access") && p.DataAccess != orm {
				return fmt.Errorf("--orm and --data-access disagree")
			}
			p.DataAccess = orm
		}
		if interactive {
			return wizard.Run(cmd.Context(), in, out, wizard.State{Project: p, Parent: parent, Verify: verify}, engine)
		}
		p.Defaults()
		if err := p.Validate(); err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
		defer cancel()
		dir, err := engine.New(ctx, parent, p, verify)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "Created %s (%s, %s/%s)\n", dir, p.HTTP, p.Database, p.DataAccess)
		if p.HTTP == "gin" {
			fmt.Fprintln(out, "Gin templates are implemented; the offline development environment could not compile their uncached dependency. Run gbme doctor --project", dir, "--verify.")
		}
		if !verify {
			fmt.Fprintln(out, "Source formatted. Dependency/build validation was skipped; use --verify when dependencies are available.")
		}
		fmt.Fprintf(out, "Next: cd %s\n      go mod tidy\n      cp .env.example .env\n      set -a; . ./.env; set +a\n", dir)
		if p.Database != "none" {
			fmt.Fprintln(out, "      make migrate-up  # after starting PostgreSQL")
		}
		fmt.Fprintln(out, "      go run ./cmd/server")
		return nil
	}}
	f := newCmd.Flags()
	f.StringVar(&p.HTTP, "http", "", "HTTP framework: stdlib, gin, fiber (default stdlib)")
	f.StringVar(&p.Database, "database", "", "Database: none, postgres (default none)")
	f.StringVar(&p.DataAccess, "data-access", "", "Data access: none, sql (inferred from database)")
	f.StringVar(&orm, "orm", "", "Alias for --data-access; GORM is not yet implemented")
	f.StringVar(&p.Module, "module", "", "Go module path (default project name)")
	f.StringVar(&parent, "dir", ".", "Existing parent directory for the new project")
	f.StringVar(&preset, "preset", "", "Curated preset: full (Fiber, PostgreSQL/sql, Redis, Docker)")
	f.BoolVar(&p.Redis, "redis", false, "Include Redis client and cache example")
	f.BoolVar(&p.Docker, "docker", false, "Include Dockerfile and Compose services")
	f.BoolVar(&interactive, "interactive", false, "Launch the terminal wizard with these flags as initial choices")
	f.BoolVar(&nonInteractive, "non-interactive", false, "Never prompt; suitable for CI")
	f.BoolVar(&verify, "verify", false, "Resolve dependencies, run generated tests and build before publication")
	root.AddCommand(newCmd)
	root.AddCommand(&cobra.Command{Use: "version", Short: "Print the generator version", Args: cobra.NoArgs, Run: func(*cobra.Command, []string) { fmt.Fprintln(out, "gbme", version) }})
	root.AddCommand(&cobra.Command{Use: "list [frameworks|databases|features|presets]", Short: "List implemented choices and validation limits", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		groups := map[string]string{"frameworks": "stdlib: executable tests verified\nfiber: executable tests verified\ngin: implemented templates; executable validation requires uncached dependencies", "databases": "none/none: HTTP-only endpoints\npostgres/sql: pooled connections, repository CRUD, SQL migrations\nGORM, sqlc, SQLite, MySQL and MongoDB: planned", "features": "core: config, slog, JSON validation, request IDs, CORS, health/readiness, shutdown\nredis: optional client, cache example and integration tests\ndocker: optional multi-stage image and Compose; daemon execution needs Docker access\nresources: GET,POST,PUT,DELETE with PostgreSQL; GET,POST in-memory endpoints", "presets": "full: Fiber + PostgreSQL/sql + Redis + Docker"}
		names := []string{"frameworks", "databases", "features", "presets"}
		if len(args) > 0 {
			if _, ok := groups[args[0]]; !ok {
				return fmt.Errorf("unknown category %q", args[0])
			}
			names = args
		}
		for _, name := range names {
			fmt.Fprintf(out, "%s:\n%s\n", name, groups[name])
		}
		return nil
	}})
	add := &cobra.Command{Use: "add", Short: "Add connected handlers, DTOs, tests and routes to a gbme project"}
	for _, kind := range []string{"endpoint", "resource"} {
		kind := kind
		var fields, methods, project string
		var crud, check bool
		command := &cobra.Command{Use: kind + " NAME", Short: map[string]string{"endpoint": "Add an in-memory GET/POST HTTP example", "resource": "Add PostgreSQL-backed CRUD"}[kind], Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
			if kind == "resource" && !crud {
				return fmt.Errorf("resource generation requires --crud; use add endpoint for HTTP-only examples")
			}
			parsed, err := config.ParseFields(fields)
			if err != nil {
				return err
			}
			r := config.Resource{Name: args[0], Fields: parsed, Persistent: kind == "resource"}
			if kind == "resource" {
				r.Methods = []string{"GET", "POST", "PUT", "DELETE"}
			} else {
				for _, m := range strings.Split(methods, ",") {
					m = strings.ToUpper(strings.TrimSpace(m))
					if m != "GET" && m != "POST" {
						return fmt.Errorf("HTTP-only endpoints support GET and POST; use add resource --crud for persistence")
					}
					r.Methods = append(r.Methods, m)
				}
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
			defer cancel()
			if err = engine.Add(ctx, project, r, check); err != nil {
				return err
			}
			fmt.Fprintf(out, "Added %s %s at /api/v1/%s\n", kind, r.Name, r.Name)
			if r.Persistent {
				fmt.Fprintln(out, "Apply the new migration with make migrate-up (or docker compose run --rm migrate).")
			}
			return nil
		}}
		command.Flags().StringVar(&fields, "fields", "", "Required comma-separated name:type fields")
		command.Flags().StringVar(&project, "project", ".", "gbme project directory")
		command.Flags().BoolVar(&check, "verify", false, "Test and build a staged copy before changing the project")
		if kind == "endpoint" {
			command.Flags().StringVar(&methods, "method", "GET,POST", "Comma-separated HTTP methods: GET,POST")
		} else {
			command.Flags().BoolVar(&crud, "crud", false, "Generate GET,POST,PUT,DELETE persistence operations")
		}
		add.AddCommand(command)
	}
	root.AddCommand(add)
	var doctorProject string
	var doctorVerify bool
	doctor := &cobra.Command{Use: "doctor", Short: "Inspect local tools and optionally test a generated project", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		for _, tool := range []string{"go", "docker", "migrate"} {
			path, err := exec.LookPath(tool)
			if err != nil {
				fmt.Fprintf(out, "%s: unavailable%s\n", tool, map[string]string{"docker": " (optional)", "migrate": " (make uses pinned go run)"}[tool])
			} else {
				fmt.Fprintf(out, "%s: %s\n", tool, path)
			}
		}
		if doctorProject != "" {
			m, err := generator.Load(doctorProject)
			if err != nil {
				return err
			}
			fmt.Fprintf(out, "Project: %s, %s, %s/%s, %d resources\n", m.Project.Name, m.Project.HTTP, m.Project.Database, m.Project.DataAccess, len(m.Resources))
		}
		if doctorVerify {
			if doctorProject == "" {
				return fmt.Errorf("--verify requires --project PATH")
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
			defer cancel()
			if err := generator.Verify(ctx, doctorProject); err != nil {
				return err
			}
			fmt.Fprintln(out, "Generated tests and build passed.")
		}
		return nil
	}}
	doctor.Flags().StringVar(&doctorProject, "project", "", "Inspect a generated project manifest")
	doctor.Flags().BoolVar(&doctorVerify, "verify", false, "Run go mod tidy, tests and build in the project")
	root.AddCommand(doctor)
	return root
}
