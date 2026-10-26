package cli

import (
	"bytes"
	"go-boiletplate-made-easy/internal/generator"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func execute(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := New("test-version", strings.NewReader(""), &out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}
func TestVersionAndHelp(t *testing.T) {
	for _, args := range [][]string{{"version"}, {"--help"}, {"list"}, {"list", "frameworks"}, {"doctor"}} {
		out, err := execute(t, args...)
		if err != nil || out == "" {
			t.Fatalf("%v: %s %v", args, out, err)
		}
	}
}
func TestCLIProjectDefaults(t *testing.T) {
	root := t.TempDir()
	out, err := execute(t, "new", "test-api", "--dir", root, "--non-interactive")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Created") {
		t.Fatal(out)
	}
	m, err := generator.Load(filepath.Join(root, "test-api"))
	if err != nil {
		t.Fatal(err)
	}
	if m.Project.HTTP != "stdlib" || m.Project.Database != "none" {
		t.Fatal(m.Project)
	}
}
func TestFullPreset(t *testing.T) {
	root := t.TempDir()
	if _, err := execute(t, "new", "full", "--dir", root, "--preset", "full", "--non-interactive"); err != nil {
		t.Fatal(err)
	}
	m, err := generator.Load(filepath.Join(root, "full"))
	if err != nil {
		t.Fatal(err)
	}
	if m.Project.HTTP != "fiber" || m.Project.Database != "postgres" || !m.Project.Redis || !m.Project.Docker {
		t.Fatal(m.Project)
	}
}
func TestInvalidFlags(t *testing.T) {
	for _, args := range [][]string{{"new"}, {"new", "name", "--unknown"}, {"new", "name", "--http", "echo"}, {"new", "name", "--database", "postgres", "--orm", "gorm"}, {"new", "name", "--orm", "sql", "--data-access", "none"}, {"new", "name", "--interactive", "--non-interactive"}, {"new", "name", "--preset", "bad"}, {"add", "resource", "products", "--fields", "name:string"}, {"add", "endpoint", "users", "--fields", "name:string", "--method", "PATCH"}, {"list", "bad"}, {"doctor", "--verify"}} {
		if _, err := execute(t, args...); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}
func TestWizardRequiresTerminal(t *testing.T) {
	for _, args := range [][]string{nil, {"new", "wizard", "--interactive"}} {
		out, err := execute(t, args...)
		if err == nil || !strings.Contains(err.Error(), "interactive terminal") || out != "" {
			t.Fatalf("%v: %q %v", args, out, err)
		}
	}
}

func TestExplicitNewDoesNotPrompt(t *testing.T) {
	for _, flags := range [][]string{nil, {"--http", "gin", "--database", "postgres", "--docker"}} {
		dir := t.TempDir()
		args := append([]string{"new", "api", "--dir", dir}, flags...)
		out, err := execute(t, args...)
		if err != nil || !strings.Contains(out, "Created") {
			t.Fatalf("%v: %s %v", args, out, err)
		}
		if _, err := generator.Load(filepath.Join(dir, "api")); err != nil {
			t.Fatal(err)
		}
	}
}

func TestWizardNonTerminalLeavesNoFiles(t *testing.T) {
	dir := t.TempDir()
	_, err := execute(t, "new", "eof", "--dir", dir, "--interactive")
	if err == nil {
		t.Fatal("expected terminal error")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatal("partial output")
	}
}
func TestEndpointCommand(t *testing.T) {
	dir := t.TempDir()
	if _, err := execute(t, "new", "api", "--dir", dir, "--non-interactive"); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(dir, "api")
	if _, err := execute(t, "add", "endpoint", "users", "--project", root, "--fields", "name:string,email:string"); err != nil {
		t.Fatal(err)
	}
	m, err := generator.Load(root)
	if err != nil || len(m.Resources) != 1 {
		t.Fatalf("%v %v", m, err)
	}
}
