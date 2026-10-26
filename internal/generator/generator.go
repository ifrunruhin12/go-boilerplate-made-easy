package generator

import (
	"bytes"
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go-boiletplate-made-easy/internal/config"
	"go/format"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/template"
)

//go:embed templates
var templates embed.FS

const ManifestName = ".gbme.json"

type OwnedFile struct {
	Feature string `json:"feature"`
	SHA256  string `json:"sha256"`
}
type Feature struct {
	Dependencies []string `json:"go_dependencies,omitempty"`
	Requires     []string `json:"requires,omitempty"`
	SharedFiles  []string `json:"shared_files,omitempty"`
}
type Manifest struct {
	Schema    int                  `json:"schema"`
	Generator string               `json:"generator"`
	Project   config.Project       `json:"project"`
	Resources []config.Resource    `json:"resources"`
	Files     map[string]OwnedFile `json:"files"`
	Features  map[string]Feature   `json:"features"`
}
type artifact struct {
	body    []byte
	feature string
}
type Data struct {
	config.Project
	Resources []config.Resource
	Resource  config.Resource
	Version   int
}
type Engine struct{ Version string }

func digest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func render(name string, d Data) ([]byte, error) {
	b, err := templates.ReadFile("templates/" + name + ".tmpl")
	if err != nil {
		return nil, err
	}
	t, err := template.New(name).Funcs(template.FuncMap{"join": strings.Join, "add": func(a, b int) int { return a + b }}).Option("missingkey=error").Parse(string(b))
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err = t.Execute(&out, d); err != nil {
		return nil, fmt.Errorf("render %s: %w", name, err)
	}
	return out.Bytes(), nil
}
func add(files map[string]artifact, path, name, feature string, d Data) error {
	b, err := render(name, d)
	if err != nil {
		return err
	}
	if strings.HasSuffix(path, ".go") {
		b, err = format.Source(b)
		if err != nil {
			return fmt.Errorf("format %s: %w", path, err)
		}
	}
	files[path] = artifact{b, feature}
	return nil
}
func (e Engine) compose(p config.Project, resources []config.Resource) (map[string]artifact, error) {
	d := Data{Project: p, Resources: resources}
	files := map[string]artifact{}
	specs := [][3]string{
		{"go.mod", "base/go.mod", "core"}, {"cmd/server/main.go", "base/main.go", "core"},
		{"internal/config/config.go", "base/config.go", "core"}, {"internal/config/config_test.go", "base/config_test.go", "core"},
		{"internal/api/api.go", "base/api.go", "core"}, {"internal/api/api_test.go", "base/api_test.go", "core"},
		{"README.md", "base/README.md", "core"}, {".env.example", "base/env", "core"}, {".gitignore", "base/gitignore", "core"}, {"GBME-LICENSE.txt", "base/license", "core"}, {"Makefile", "base/Makefile", "core"},
		{"internal/server/server.go", p.HTTP + "/server.go", "http"},
		{"internal/server/server_test.go", p.HTTP + "/server_test.go", "http"},
		{"internal/server/resources_gen.go", p.HTTP + "/routes.go", "http"},
	}
	if p.Database == "postgres" {
		specs = append(specs, [3]string{"internal/database/database.go", "postgres/database.go", "database"}, [3]string{"internal/database/database_test.go", "postgres/database_test.go", "database"}, [3]string{"migrations/000001_notes.up.sql", "postgres/notes.up.sql", "database"}, [3]string{"migrations/000001_notes.down.sql", "postgres/notes.down.sql", "database"}, [3]string{"internal/database/notes.go", "postgres/notes.go", "database"})
	}
	if p.Redis {
		specs = append(specs, [3]string{"internal/cache/cache.go", "redis/cache.go", "redis"}, [3]string{"internal/cache/cache_test.go", "redis/cache_test.go", "redis"})
	}
	if p.Docker {
		specs = append(specs, [3]string{"Dockerfile", "docker/Dockerfile", "docker"}, [3]string{"compose.yaml", "docker/compose.yaml", "docker"}, [3]string{".dockerignore", "docker/dockerignore", "docker"})
	}
	for _, s := range specs {
		if err := add(files, s[0], s[1], s[2], d); err != nil {
			return nil, err
		}
	}
	for i, r := range resources {
		d.Resource = r
		d.Version = r.MigrationVersion
		if d.Version == 0 {
			d.Version = i + 2
		}
		feature := "resource:" + r.Name
		for _, s := range [][2]string{{"model.go", "resource/model.go"}, {"service.go", "resource/service.go"}, {"service_test.go", "resource/service_test.go"}, {"handler.go", p.HTTP + "/handler.go"}, {"handler_test.go", p.HTTP + "/handler_test.go"}} {
			if err := add(files, "internal/"+r.Name+"/"+s[0], s[1], feature, d); err != nil {
				return nil, err
			}
		}
		if r.Persistent {
			if err := add(files, "internal/"+r.Name+"/repository.go", "resource/repository.go", feature, d); err != nil {
				return nil, err
			}
			if err := add(files, "internal/"+r.Name+"/repository_test.go", "resource/repository_test.go", feature, d); err != nil {
				return nil, err
			}
			for _, direction := range []string{"up", "down"} {
				path := fmt.Sprintf("migrations/%06d_%s.%s.sql", d.Version, r.Name, direction)
				if err := add(files, path, "resource/"+direction+".sql", feature, d); err != nil {
					return nil, err
				}
			}
		}
	}
	return files, nil
}
func manifest(p config.Project, rs []config.Resource, v string, files map[string]artifact) Manifest {
	m := Manifest{Schema: 1, Generator: v, Project: p, Resources: rs, Files: map[string]OwnedFile{}, Features: map[string]Feature{"core": {}, "http": {Requires: []string{"core"}}}}
	if p.HTTP == "gin" {
		m.Features["http"] = Feature{Dependencies: []string{"github.com/gin-gonic/gin"}, Requires: []string{"core"}}
	}
	if p.HTTP == "fiber" {
		m.Features["http"] = Feature{Dependencies: []string{"github.com/gofiber/fiber/v2"}, Requires: []string{"core"}}
	}
	if p.Database == "postgres" {
		m.Features["database"] = Feature{Dependencies: []string{"github.com/jackc/pgx/v5"}, Requires: []string{"core"}}
	}
	if p.Redis {
		m.Features["redis"] = Feature{Dependencies: []string{"github.com/redis/go-redis/v9"}, Requires: []string{"core"}}
	}
	if p.Docker {
		m.Features["docker"] = Feature{Requires: []string{"core"}}
	}
	for _, r := range rs {
		req := []string{"http"}
		if r.Persistent {
			req = append(req, "database")
		}
		m.Features["resource:"+r.Name] = Feature{Requires: req}
	}
	for _, name := range []string{"database", "redis"} {
		if feature, ok := m.Features[name]; ok {
			feature.SharedFiles = []string{"cmd/server/main.go", "internal/config/config.go", "internal/config/config_test.go", "internal/server/server.go", "internal/server/server_test.go", "go.mod", ".env.example", "README.md"}
			if p.Docker {
				feature.SharedFiles = append(feature.SharedFiles, "Dockerfile", "compose.yaml")
			}
			m.Features[name] = feature
		}
	}
	for _, r := range rs {
		feature := m.Features["resource:"+r.Name]
		feature.SharedFiles = []string{"internal/server/resources_gen.go"}
		m.Features["resource:"+r.Name] = feature
	}
	for path, a := range files {
		m.Files[path] = OwnedFile{Feature: a.feature, SHA256: digest(a.body)}
	}
	return m
}
func writeFiles(root string, files map[string]artifact, m Manifest) error {
	for path, a := range files {
		target := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(target, a.body, 0644); err != nil {
			return err
		}
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, ManifestName), append(b, '\n'), 0644)
}
func Verify(ctx context.Context, dir string) error {
	for _, args := range [][]string{{"mod", "tidy"}, {"test", "./..."}, {"build", "-buildvcs=false", "./..."}} {
		cmd := exec.CommandContext(ctx, "go", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOWORK=off")
		b, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("go %s failed: %w\n%s", strings.Join(args, " "), err, b)
		}
	}
	return nil
}
func (e Engine) New(ctx context.Context, parent string, p config.Project, verify bool) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	p.Defaults()
	if err := p.Validate(); err != nil {
		return "", err
	}
	parent, err := filepath.Abs(parent)
	if err != nil {
		return "", err
	}
	target := filepath.Join(parent, p.Name)
	if _, err := os.Lstat(target); !os.IsNotExist(err) {
		return "", fmt.Errorf("destination %s exists or cannot be inspected; choose a new project name", target)
	}
	stage, err := os.MkdirTemp(parent, ".gbme-stage-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(stage)
	files, err := e.compose(p, nil)
	if err != nil {
		return "", err
	}
	m := manifest(p, nil, e.Version, files)
	if err = writeFiles(stage, files, m); err != nil {
		return "", err
	}
	if verify {
		if err = Verify(ctx, stage); err != nil {
			return "", err
		}
		for _, path := range []string{"go.mod", "go.sum"} {
			b, err := os.ReadFile(filepath.Join(stage, path))
			if err == nil {
				m.Files[path] = OwnedFile{Feature: "core", SHA256: digest(b)}
			}
		}
		b, _ := json.MarshalIndent(m, "", "  ")
		if err = os.WriteFile(filepath.Join(stage, ManifestName), append(b, '\n'), 0644); err != nil {
			return "", err
		}
	}
	// Reserve the destination exclusively. Publish with exclusive file creation,
	// and clean up only files/directories created by this invocation on failure.
	if err = ctx.Err(); err != nil {
		return "", err
	}
	if err = os.Mkdir(target, 0755); err != nil {
		return "", err
	}
	var published []string
	directories := []string{target}
	cleanup := func() {
		for _, path := range published {
			_ = os.Remove(path)
		}
		for i := len(directories) - 1; i >= 0; i-- {
			_ = os.Remove(directories[i])
		}
	}
	err = filepath.WalkDir(stage, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, err := filepath.Rel(stage, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		dest := filepath.Join(target, rel)
		if entry.IsDir() {
			if err = os.Mkdir(dest, 0755); err != nil {
				return err
			}
			directories = append(directories, dest)
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		f, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if err != nil {
			return err
		}
		published = append(published, dest)
		_, err = f.Write(body)
		closeErr := f.Close()
		if err != nil {
			return err
		}
		return closeErr
	})
	if err != nil {
		cleanup()
		return "", err
	}
	return target, nil
}
func Load(root string) (Manifest, error) {
	var m Manifest
	b, err := os.ReadFile(filepath.Join(root, ManifestName))
	if err != nil {
		return m, fmt.Errorf("not a gbme project: %w", err)
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err = dec.Decode(&m); err != nil {
		return m, err
	}
	if err = dec.Decode(new(any)); err != io.EOF {
		return m, fmt.Errorf("manifest must contain one JSON object")
	}
	if m.Files == nil || m.Features == nil {
		return m, fmt.Errorf("manifest is missing file ownership or feature metadata")
	}
	if m.Schema != 1 {
		return m, fmt.Errorf("unsupported manifest schema %d", m.Schema)
	}
	if err = m.Project.Validate(); err != nil {
		return m, err
	}
	seen := map[string]bool{}
	for _, r := range m.Resources {
		if err = r.Validate(); err != nil {
			return m, err
		}
		if seen[r.Name] {
			return m, fmt.Errorf("duplicate resource in manifest: %s", r.Name)
		}
		seen[r.Name] = true
		if r.Persistent && m.Project.Database == "none" {
			return m, fmt.Errorf("persistent resource requires a database")
		}
		if r.MigrationVersion < 0 {
			return m, fmt.Errorf("invalid migration version")
		}
	}
	for path, owned := range m.Files {
		if filepath.IsAbs(path) || filepath.Clean(path) != path || strings.HasPrefix(path, "..") || strings.Contains(path, `\`) {
			return m, fmt.Errorf("invalid owned file path %q", path)
		}
		if decoded, err := hex.DecodeString(owned.SHA256); err != nil || len(decoded) != 32 {
			return m, fmt.Errorf("invalid ownership hash for %s", path)
		}
	}
	return m, nil
}
func (e Engine) Add(ctx context.Context, root string, r config.Resource, verify bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := r.Validate(); err != nil {
		return err
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	lock, err := os.OpenFile(filepath.Join(root, ".gbme.lock"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("cannot lock project; another generation may be running: %w", err)
	}
	lock.Close()
	defer os.Remove(filepath.Join(root, ".gbme.lock"))
	m, err := Load(root)
	if err != nil {
		return err
	}
	oldManifest, err := os.ReadFile(filepath.Join(root, ManifestName))
	if err != nil {
		return err
	}
	oldRoutes, err := os.ReadFile(filepath.Join(root, "internal/server/resources_gen.go"))
	if err != nil {
		return err
	}
	if r.Persistent && m.Project.Database == "none" {
		return fmt.Errorf("database-backed CRUD requires --database postgres --data-access sql at project creation; use add endpoint for an in-memory example")
	}
	for _, existing := range m.Resources {
		if existing.Name == r.Name {
			return fmt.Errorf("resource %q already exists", r.Name)
		}
	}
	if r.Persistent {
		entries, err := os.ReadDir(filepath.Join(root, "migrations"))
		if err != nil {
			return err
		}
		highest := 1
		for _, entry := range entries {
			name := entry.Name()
			if !strings.HasSuffix(name, ".up.sql") && !strings.HasSuffix(name, ".down.sql") {
				continue
			}
			version, err := strconv.Atoi(strings.SplitN(name, "_", 2)[0])
			if err != nil || version < 1 {
				return fmt.Errorf("invalid migration name %q", name)
			}
			if version > highest {
				highest = version
			}
		}
		r.MigrationVersion = highest + 1
	}
	rs := append(append([]config.Resource{}, m.Resources...), r)
	files, err := e.compose(m.Project, rs)
	if err != nil {
		return err
	}
	updates := map[string]artifact{}
	route := "internal/server/resources_gen.go"
	for path, a := range files {
		if path == route || strings.HasPrefix(path, "internal/"+r.Name+"/") || (strings.HasPrefix(path, "migrations/") && strings.Contains(path, "_"+r.Name+".")) {
			updates[path] = a
		}
	}
	for path := range updates {
		// Reject symlinks at every ancestor, including a user-replaced resource directory.
		parts := strings.Split(filepath.ToSlash(path), "/")
		cur := root
		for _, part := range parts {
			cur = filepath.Join(cur, part)
			info, err := os.Lstat(cur)
			if err == nil && info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("refusing symlink %s", cur)
			}
			if err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		b, err := os.ReadFile(filepath.Join(root, path))
		if path == route {
			owner, ok := m.Files[path]
			if err != nil || !ok || digest(b) != owner.SHA256 {
				return fmt.Errorf("%s was modified; move custom registrations to server.go before regenerating", path)
			}
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("refusing to overwrite %s", path)
		}
	}
	stage, err := os.MkdirTemp(filepath.Dir(root), ".gbme-add-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	if verify {
		err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			if entry.IsDir() {
				if entry.Name() == ".git" {
					return filepath.SkipDir
				}
				return os.MkdirAll(filepath.Join(stage, rel), 0755)
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("cannot verify project containing symlink %s", rel)
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(stage, rel), b, info.Mode().Perm())
		})
		if err != nil {
			return err
		}
	}
	m.Resources = rs
	m.Generator = e.Version
	next := manifest(m.Project, rs, e.Version, files)
	m.Features = next.Features
	for path, a := range updates {
		m.Files[path] = OwnedFile{a.feature, digest(a.body)}
	}
	if err = writeFiles(stage, updates, m); err != nil {
		return err
	}
	if verify {
		if err = Verify(ctx, stage); err != nil {
			return err
		}
	}
	// Roll back published files and the registry if a write fails.
	if err = ctx.Err(); err != nil {
		return err
	}
	for path, original := range map[string][]byte{route: oldRoutes, ManifestName: oldManifest} {
		current, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			return err
		}
		if !bytes.Equal(current, original) {
			return fmt.Errorf("%s changed during verification; no files published", path)
		}
	}
	var created []string
	var dirs []string
	registryChanged := false
	rollback := func() {
		for _, p := range created {
			os.Remove(filepath.Join(root, p))
		}
		for i := len(dirs) - 1; i >= 0; i-- {
			os.Remove(dirs[i])
		}
		if registryChanged {
			_ = os.WriteFile(filepath.Join(root, route), oldRoutes, 0644)
		}
	}
	paths := make([]string, 0, len(updates))
	for path := range updates {
		if path != route {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	for _, path := range paths {
		dest := filepath.Join(root, path)
		dir := filepath.Dir(dest)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			if err = os.MkdirAll(dir, 0755); err != nil {
				rollback()
				return err
			}
			dirs = append(dirs, dir)
		}
		f, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if err != nil {
			rollback()
			return err
		}
		created = append(created, path)
		_, err = f.Write(updates[path].body)
		closeErr := f.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			rollback()
			return err
		}
	}
	if err = os.Rename(filepath.Join(stage, route), filepath.Join(root, route)); err != nil {
		rollback()
		return err
	}
	registryChanged = true
	if err = os.Rename(filepath.Join(stage, ManifestName), filepath.Join(root, ManifestName)); err != nil {
		rollback()
		return err
	}
	return nil
}
