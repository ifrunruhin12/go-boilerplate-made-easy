package generator

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"go-boiletplate-made-easy/internal/config"
)

func TestAddPreservesConcurrentSharedFileEdits(t *testing.T) {
	for _, path := range []string{"openapi.json", "internal/server/openapi.go", "internal/server/resources_gen.go", ManifestName} {
		t.Run(path, func(t *testing.T) {
			e, root, originals := openAPIProject(t)
			t.Setenv("GBME_MUTATE_PATH", filepath.Join(root, path))
			fakeGo(t, "printf '\\n// user edit\\n' >> \"$GBME_MUTATE_PATH\"")
			err := e.Add(context.Background(), root, endpointResource(), true)
			if err == nil || !strings.Contains(err.Error(), "changed during verification") {
				t.Fatalf("expected concurrent edit rejection, got %v", err)
			}
			for name, original := range originals {
				current, err := os.ReadFile(filepath.Join(root, name))
				if err != nil {
					t.Fatal(err)
				}
				if name == path {
					if !bytes.HasPrefix(current, original) || !bytes.Contains(current, []byte("user edit")) {
						t.Fatal("concurrent user edit was lost")
					}
				} else if !bytes.Equal(current, original) {
					t.Fatalf("%s was partially published", name)
				}
			}
			assertNoResource(t, root)
			if _, err := os.Stat(filepath.Join(root, ".gbme.lock")); !os.IsNotExist(err) {
				t.Fatal("project lock was not released")
			}
		})
	}
}

func TestAddRejectsModifiedOpenAPIFiles(t *testing.T) {
	for _, path := range []string{"openapi.json", "internal/server/openapi.go"} {
		t.Run(path, func(t *testing.T) {
			e, root, _ := openAPIProject(t)
			edit := []byte("user edit")
			if err := os.WriteFile(filepath.Join(root, path), edit, 0644); err != nil {
				t.Fatal(err)
			}
			if err := e.Add(context.Background(), root, endpointResource(), false); err == nil || !strings.Contains(err.Error(), "was modified") {
				t.Fatalf("expected ownership rejection, got %v", err)
			}
			current, err := os.ReadFile(filepath.Join(root, path))
			if err != nil || !bytes.Equal(current, edit) {
				t.Fatal("user edit was lost")
			}
			assertNoResource(t, root)
		})
	}
}

func TestAddRestoresOpenAPIAfterPublicationFailure(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("requires Unix directory permissions for an unprivileged user")
	}
	e, root, originals := openAPIProject(t)
	embedded := filepath.Join(root, "internal/server/openapi.go")
	if err := os.Chmod(embedded, 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GBME_MUTATE_DIR", root)
	// Resource creation and the first OpenAPI rename can succeed in internal/,
	// but replacing openapi.json in the read-only project root must fail.
	fakeGo(t, "chmod 0555 \"$GBME_MUTATE_DIR\"")
	t.Cleanup(func() { os.Chmod(root, 0755) })
	err := e.Add(context.Background(), root, endpointResource(), true)
	if err == nil || !strings.Contains(err.Error(), "openapi.json") {
		t.Fatalf("expected OpenAPI publication failure, got %v", err)
	}
	if err := os.Chmod(root, 0755); err != nil {
		t.Fatal(err)
	}
	for path, original := range originals {
		current, err := os.ReadFile(filepath.Join(root, path))
		if err != nil || !bytes.Equal(current, original) {
			t.Fatalf("failed publication did not restore %s: %v", path, err)
		}
	}
	info, err := os.Stat(embedded)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("rollback did not preserve original file permissions")
	}
	assertNoResource(t, root)
}

func TestAddRejectsSymlinkIntroducedDuringVerification(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a Unix shell and symlinks")
	}
	e, root, _ := openAPIProject(t)
	path := filepath.Join(root, "openapi.json")
	t.Setenv("GBME_MUTATE_PATH", path)
	fakeGo(t, "if [ ! -L \"$GBME_MUTATE_PATH\" ]; then\n mv \"$GBME_MUTATE_PATH\" \"$GBME_MUTATE_PATH.original\"\n ln -s \"$GBME_MUTATE_PATH.original\" \"$GBME_MUTATE_PATH\"\nfi")
	if err := e.Add(context.Background(), root, endpointResource(), true); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("expected symlink rejection, got %v", err)
	}
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("verification replaced the user symlink")
	}
	assertNoResource(t, root)
}

func openAPIProject(t *testing.T) (Engine, string, map[string][]byte) {
	t.Helper()
	e := Engine{}
	root, err := e.New(context.Background(), t.TempDir(), config.Project{Name: "api", OpenAPI: true}, false)
	if err != nil {
		t.Fatal(err)
	}
	originals := map[string][]byte{}
	for _, path := range []string{"openapi.json", "internal/server/openapi.go", "internal/server/resources_gen.go", ManifestName} {
		originals[path], err = os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
	}
	return e, root, originals
}

func endpointResource() config.Resource {
	r := testResource(false)
	r.Methods = []string{"GET", "POST"}
	return r
}

func fakeGo(t *testing.T, script string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("verification fixture uses a Unix shell")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go"), []byte("#!/bin/sh\nset -e\n"+script+"\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func assertNoResource(t *testing.T, root string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(root, "internal/products")); !os.IsNotExist(err) {
		t.Fatal("failed addition left a resource directory")
	}
}
