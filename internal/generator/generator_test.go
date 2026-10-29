package generator

import (
	"context"
	"go-boiletplate-made-easy/internal/config"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testResource(persistent bool) config.Resource {
	return config.Resource{Name: "products", Fields: []config.Field{{Name: "name", Type: "string"}, {Name: "price", Type: "float64"}, {Name: "quantity", Type: "int64"}, {Name: "active", Type: "bool"}}, Methods: []string{"GET", "POST", "PUT", "DELETE"}, Persistent: persistent, MigrationVersion: 2}
}
func TestTemplateMatrix(t *testing.T) {
	e := Engine{Version: "test"}
	for _, http := range []string{"stdlib", "gin", "fiber"} {
		for _, db := range []string{"none", "postgres"} {
			for _, optional := range []bool{false, true} {
				p := config.Project{Name: "matrix", HTTP: http, Database: db, Redis: optional, Docker: optional}
				p.Defaults()
				r := testResource(db == "postgres")
				files, err := e.compose(p, []config.Resource{r})
				if err != nil {
					t.Fatalf("%+v: %v", p, err)
				}
				for path, a := range files {
					if strings.HasSuffix(path, ".go") {
						if _, err := parser.ParseFile(token.NewFileSet(), path, a.body, parser.AllErrors); err != nil {
							t.Fatalf("%s: %v", path, err)
						}
					}
				}
				if _, ok := files["Dockerfile"]; ok != optional {
					t.Fatal("Docker composition")
				}
				if _, ok := files["internal/cache/cache.go"]; ok != optional {
					t.Fatal("Redis composition")
				}
				if _, ok := files["internal/database/database.go"]; ok != (db == "postgres") {
					t.Fatal("database composition")
				}
				server := string(files["internal/server/server.go"].body)
				if !strings.Contains(server, map[string]string{"stdlib": "http.NewServeMux", "gin": "gin.New", "fiber": "fiber.New"}[http]) {
					t.Fatal("wrong server")
				}
				mod := string(files["go.mod"].body)
				if !optional && strings.Contains(mod, "go-redis") {
					t.Fatal("unselected Redis dependency")
				}
			}
		}
	}
}

func TestPostgresComposeHealthcheckUsesServiceDNS(t *testing.T) {
	e := Engine{Version: "test"}
	p := config.Project{Name: "api", Database: "postgres", Docker: true}
	p.Defaults()
	files, err := e.compose(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	compose := string(files["compose.yaml"].body)
	if !strings.Contains(compose, `pg_isready -h db -U app -d app`) {
		t.Fatal("PostgreSQL healthcheck does not validate Compose DNS reachability")
	}
}

func TestGenerationAndDuplicates(t *testing.T) {
	root := t.TempDir()
	e := Engine{Version: "test"}
	p := config.Project{Name: "api"}
	dir, err := e.New(context.Background(), root, p, false)
	if err != nil {
		t.Fatal(err)
	}
	m, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	for path, owned := range m.Files {
		b, err := os.ReadFile(filepath.Join(dir, path))
		if err != nil || digest(b) != owned.SHA256 {
			t.Fatalf("ownership: %s %v", path, err)
		}
	}
	if _, err = e.New(context.Background(), root, p, false); err == nil {
		t.Fatal("overwrote project")
	}
	if err = os.WriteFile(filepath.Join(dir, "personal.txt"), []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	r := testResource(false)
	r.Methods = []string{"GET", "POST"}
	if err = e.Add(context.Background(), dir, r, false); err != nil {
		t.Fatal(err)
	}
	if err = e.Add(context.Background(), dir, r, false); err == nil {
		t.Fatal("accepted duplicate")
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "personal.txt")); string(b) != "keep" {
		t.Fatal("lost unrelated file")
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "internal/server/resources_gen.go")); !strings.Contains(string(b), "products.NewHandler") {
		t.Fatal("unregistered endpoint")
	}
}
func TestRefuseModifiedRegistry(t *testing.T) {
	root := t.TempDir()
	e := Engine{}
	dir, err := e.New(context.Background(), root, config.Project{Name: "api"}, false)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "internal/server/resources_gen.go")
	if err = os.WriteFile(path, []byte("custom code"), 0644); err != nil {
		t.Fatal(err)
	}
	if err = e.Add(context.Background(), dir, testResource(false), false); err == nil {
		t.Fatal("overwrote modified routes")
	}
	if _, err = os.Stat(filepath.Join(dir, "internal/products")); !os.IsNotExist(err) {
		t.Fatal("partial resource")
	}
	b, _ := os.ReadFile(path)
	if string(b) != "custom code" {
		t.Fatal("lost modifications")
	}
}
func TestPathsAndSymlinks(t *testing.T) {
	e := Engine{}
	root := t.TempDir()
	for _, name := range []string{"../escape", "a/b", "..", "."} {
		if _, err := e.New(context.Background(), root, config.Project{Name: name}, false); err == nil {
			t.Fatal(name)
		}
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
		t.Fatal(err)
	}
	if _, err := e.New(context.Background(), root, config.Project{Name: "linked"}, false); err == nil {
		t.Fatal("overwrote symlink")
	}
	dir, err := e.New(context.Background(), root, config.Project{Name: "api"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(outside, filepath.Join(dir, "internal/products")); err != nil {
		t.Fatal(err)
	}
	if err = e.Add(context.Background(), dir, testResource(false), false); err == nil {
		t.Fatal("followed resource symlink")
	}
	entries, _ := os.ReadDir(outside)
	if len(entries) != 0 {
		t.Fatal("wrote outside project")
	}
}
func TestVerificationFailureCleansOutput(t *testing.T) {
	root := t.TempDir()
	tools := t.TempDir()
	if err := os.WriteFile(filepath.Join(tools, "go"), []byte("#!/bin/sh\nprintf 'intentional failure\n'\nexit 1\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", tools)
	e := Engine{}
	if _, err := e.New(context.Background(), root, config.Project{Name: "fail"}, true); err == nil {
		t.Fatal("accepted failed verification")
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 0 {
		t.Fatal("left partial output")
	}
}
func TestAddVerificationFailurePreservesProject(t *testing.T) {
	e := Engine{}
	dir, err := e.New(context.Background(), t.TempDir(), config.Project{Name: "api"}, false)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(dir, ManifestName))
	tools := t.TempDir()
	os.WriteFile(filepath.Join(tools, "go"), []byte("#!/bin/sh\nexit 1\n"), 0755)
	t.Setenv("PATH", tools)
	if err = e.Add(context.Background(), dir, testResource(false), true); err == nil {
		t.Fatal("accepted failed verification")
	}
	after, _ := os.ReadFile(filepath.Join(dir, ManifestName))
	if string(before) != string(after) {
		t.Fatal("changed manifest")
	}
	if _, err = os.Stat(filepath.Join(dir, "internal/products")); !os.IsNotExist(err) {
		t.Fatal("partial resource")
	}
}
func TestMigrationSequence(t *testing.T) {
	e := Engine{}
	dir, err := e.New(context.Background(), t.TempDir(), config.Project{Name: "api", Database: "postgres"}, false)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "migrations/000005_manual.up.sql"), []byte("SELECT 1;"), 0644)
	if err = e.Add(context.Background(), dir, testResource(true), false); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(dir, "migrations/000006_products.up.sql")); err != nil {
		t.Fatal(err)
	}
}
func TestRequireDatabaseAndLock(t *testing.T) {
	e := Engine{}
	dir, err := e.New(context.Background(), t.TempDir(), config.Project{Name: "api"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err = e.Add(context.Background(), dir, testResource(true), false); err == nil {
		t.Fatal("accepted persistent resource without database")
	}
	os.WriteFile(filepath.Join(dir, ".gbme.lock"), nil, 0600)
	if err = e.Add(context.Background(), dir, testResource(false), false); err == nil {
		t.Fatal("ignored lock")
	}
}
func TestExecutableStdlib(t *testing.T) {
	if testing.Short() {
		t.Skip("generated-project execution disabled by -short")
	}
	e := Engine{}
	dir, err := e.New(context.Background(), t.TempDir(), config.Project{Name: "executable"}, true)
	if err != nil {
		t.Fatal(err)
	}
	r := testResource(false)
	r.Methods = []string{"GET", "POST"}
	if err = e.Add(context.Background(), dir, r, true); err != nil {
		t.Fatal(err)
	}
	for _, spec := range []config.Resource{{Name: "readonly", Fields: []config.Field{{Name: "enabled", Type: "bool"}}, Methods: []string{"GET"}}, {Name: "writeonly", Fields: []config.Field{{Name: "input", Type: "string"}}, Methods: []string{"POST"}}} {
		if err = e.Add(context.Background(), dir, spec, true); err != nil {
			t.Fatal(err)
		}
	}
}
func TestExecutableMatrix(t *testing.T) {
	if os.Getenv("GBME_MATRIX") != "1" {
		t.Skip("set GBME_MATRIX=1 to resolve/build/test external-dependency projects")
	}
	for _, p := range []config.Project{{Name: "fiber-api", HTTP: "fiber"}, {Name: "postgres-api", Database: "postgres"}, {Name: "redis-api", Redis: true}, {Name: "full-api", HTTP: "fiber", Database: "postgres", Redis: true, Docker: true}, {Name: "stdlib-full", HTTP: "stdlib", Database: "postgres", Redis: true, Docker: true}} {
		t.Run(p.Name, func(t *testing.T) {
			e := Engine{}
			dir, err := e.New(context.Background(), t.TempDir(), p, true)
			if err != nil {
				t.Fatal(err)
			}
			r := testResource(p.Database == "postgres")
			if !r.Persistent {
				r.Methods = []string{"GET", "POST"}
			}
			if err = e.Add(context.Background(), dir, r, true); err != nil {
				t.Fatal(err)
			}
		})
	}
	if os.Getenv("GBME_GIN_MATRIX") == "1" {
		t.Run("gin", func(t *testing.T) {
			e := Engine{}
			dir, err := e.New(context.Background(), t.TempDir(), config.Project{Name: "gin-api", HTTP: "gin", Database: "postgres", Redis: true, Docker: true}, true)
			if err != nil {
				t.Fatal(err)
			}
			if err = e.Add(context.Background(), dir, testResource(true), true); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCanceledContextDoesNotPublish(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	root := t.TempDir()
	if _, err := (Engine{}).New(ctx, root, config.Project{Name: "canceled"}, false); err == nil {
		t.Fatal("ignored cancellation")
	}
	entries, _ := os.ReadDir(root)
	if len(entries) != 0 {
		t.Fatal("published after cancellation")
	}
}
func TestRegistryChangedDuringVerification(t *testing.T) {
	e := Engine{}
	dir, err := e.New(context.Background(), t.TempDir(), config.Project{Name: "api"}, false)
	if err != nil {
		t.Fatal(err)
	}
	tools := t.TempDir()
	path := filepath.Join(dir, "internal/server/resources_gen.go")
	t.Setenv("GBME_MUTATE_PATH", path)
	os.WriteFile(filepath.Join(tools, "go"), []byte("#!/bin/sh\nprintf '// user edit\n' > \"$GBME_MUTATE_PATH\"\n"), 0755)
	t.Setenv("PATH", tools)
	if err = e.Add(context.Background(), dir, testResource(false), true); err == nil {
		t.Fatal("overwrote concurrent edit")
	}
	b, _ := os.ReadFile(path)
	if !strings.Contains(string(b), "user edit") {
		t.Fatal("lost edit")
	}
	if _, err = os.Stat(filepath.Join(dir, "internal/products")); !os.IsNotExist(err) {
		t.Fatal("partial publication")
	}
}
func TestPublicationFailureRollsBackOnlyOwnedFiles(t *testing.T) {
	e := Engine{}
	dir, err := e.New(context.Background(), t.TempDir(), config.Project{Name: "api"}, false)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(dir, ManifestName))
	tools := t.TempDir()
	resourceDir := filepath.Join(dir, "internal/products")
	t.Setenv("GBME_MUTATE_DIR", resourceDir)
	os.WriteFile(filepath.Join(tools, "go"), []byte("#!/bin/sh\nmkdir -p \"$GBME_MUTATE_DIR\"\nprintf 'user file\n' > \"$GBME_MUTATE_DIR/model.go\"\n"), 0755)
	t.Setenv("PATH", tools+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err = e.Add(context.Background(), dir, testResource(false), true); err == nil {
		t.Fatal("overwrote raced-in file")
	}
	after, _ := os.ReadFile(filepath.Join(dir, ManifestName))
	if string(after) != string(before) {
		t.Fatal("changed manifest")
	}
	b, _ := os.ReadFile(filepath.Join(resourceDir, "model.go"))
	if string(b) != "user file\n" {
		t.Fatal("deleted user file")
	}
	entries, _ := os.ReadDir(resourceDir)
	if len(entries) != 1 {
		t.Fatalf("partial generated files: %v", entries)
	}
}
