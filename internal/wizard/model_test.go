package wizard

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"go-boiletplate-made-easy/internal/config"
	"go-boiletplate-made-easy/internal/generator"
)

type generatorFunc func(context.Context, string, config.Project, bool) (string, error)

func (f generatorFunc) New(ctx context.Context, parent string, p config.Project, verify bool) (string, error) {
	return f(ctx, parent, p, verify)
}

func testModel(t *testing.T, engine Generator) *Model {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	if engine == nil {
		engine = generator.Engine{Version: "wizard-test"}
	}
	return NewModel(ctx, State{Parent: t.TempDir()}, engine)
}

func press(m *Model, code rune) tea.Cmd {
	_, cmd := m.Update(tea.KeyPressMsg{Code: code})
	return cmd
}
func typeText(m *Model, text string) {
	for _, r := range text {
		m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}
}
func ctrlC(m *Model) tea.Cmd {
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	return cmd
}

// Execute the initial batch without a real event loop or terminal. Tick messages
// need not be scheduled again; only generator completion affects these tests.
func executeCommand(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var msgs []tea.Msg
		for _, c := range batch {
			msgs = append(msgs, executeCommand(c)...)
		}
		return msgs
	}
	return []tea.Msg{msg}
}
func complete(t *testing.T, m *Model, cmd tea.Cmd) {
	t.Helper()
	found := false
	for _, msg := range executeCommand(cmd) {
		if _, ok := msg.(generatedMsg); ok {
			found = true
			m.Update(msg)
		}
	}
	if !found {
		t.Fatal("no generator completion message")
	}
}

func reachReview(t *testing.T, m *Model) {
	t.Helper()
	for i := 0; i < 12; i++ {
		if m.step == Review {
			return
		}
		press(m, tea.KeyEnter)
		if m.err != nil {
			t.Fatal(m.err)
		}
	}
	t.Fatal("wizard did not reach review")
}

func TestNavigationAndConditionalSteps(t *testing.T) {
	m := testModel(t, nil)
	if !strings.Contains(m.View().Content, "Step 1 of 8") {
		t.Fatal(m.View().Content)
	}
	press(m, tea.KeyEnter)
	if m.err == nil || m.step != Name {
		t.Fatal("empty name accepted")
	}
	typeText(m, "billing-api")
	press(m, tea.KeyEnter)
	if m.step != Directory {
		t.Fatal(m.step)
	}
	press(m, tea.KeyEscape)
	if m.input.Value() != "billing-api" {
		t.Fatal("name lost on back")
	}
	press(m, tea.KeyBackspace)
	if m.step != Name || m.state.Project.Name != "billing-ap" {
		t.Fatal("backspace must edit text")
	}
	typeText(m, "i")
	press(m, tea.KeyEnter)
	press(m, tea.KeyEnter)
	if m.step != HTTP {
		t.Fatal(m.step)
	}
	press(m, tea.KeyDown)
	if m.state.Project.HTTP != "gin" {
		t.Fatal(m.state.Project)
	}
	press(m, tea.KeyDown)
	press(m, tea.KeyEnter)
	press(m, tea.KeyDown)
	if m.state.Project.Database != "postgres" || m.state.Project.DataAccess != "sql" {
		t.Fatal(m.state.Project)
	}
	if !strings.Contains(m.View().Content, "Step 4 of 9") {
		t.Fatal(m.View().Content)
	}
	press(m, tea.KeyEnter)
	if m.step != DataAccess {
		t.Fatal("missing SQL step")
	}
	choices, _ := m.choices()
	if len(choices) != 1 || choices[0].Value != "sql" {
		t.Fatal(choices)
	}
	press(m, tea.KeyBackspace)
	press(m, tea.KeyUp)
	press(m, tea.KeyEnter)
	if m.step != Redis || m.state.Project.DataAccess != "none" {
		t.Fatal("no database must skip SQL")
	}
	press(m, tea.KeySpace)
	press(m, tea.KeyEnter)
	press(m, tea.KeyRight)
	press(m, tea.KeyEscape)
	if !m.state.Project.Redis || !m.state.Project.Docker || m.state.Project.HTTP != "fiber" {
		t.Fatal("selections were lost")
	}
	reachReview(t, m)
	if _, err := os.Stat(filepath.Join(m.state.Parent, "billing-api")); !os.IsNotExist(err) {
		t.Fatal("generated before confirmation")
	}
	if !strings.Contains(m.View().Content, "billing-api") {
		t.Fatal(m.View().Content)
	}
}

func TestNameAndPathValidation(t *testing.T) {
	for _, name := range []string{"", "My API", "../escape", "api/child", strings.Repeat("a", 65)} {
		m := testModel(t, nil)
		typeText(m, name)
		press(m, tea.KeyEnter)
		if m.err == nil || m.step != Name {
			t.Fatalf("accepted %q", name)
		}
	}
	root := t.TempDir()
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, parent := range []string{"", filepath.Join(root, "missing"), file, "~someone", "a\x00b", "a\nb"} {
		s := State{Project: config.Project{Name: "api"}, Parent: parent}
		if _, err := s.Validate(); err == nil {
			t.Fatalf("accepted path %q", parent)
		}
	}
	for _, targetType := range []string{"file", "directory", "symlink"} {
		t.Run(targetType, func(t *testing.T) {
			parent := t.TempDir()
			target := filepath.Join(parent, "api")
			var err error
			switch targetType {
			case "file":
				err = os.WriteFile(target, []byte("keep"), 0600)
			case "directory":
				err = os.Mkdir(target, 0700)
			case "symlink":
				err = os.Symlink(filepath.Join(parent, "missing"), target)
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := (State{Project: config.Project{Name: "api"}, Parent: parent}).Validate(); err == nil {
				t.Fatal("existing destination accepted")
			}
		})
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	for _, parent := range []string{"~", "~/"} {
		target, err := (State{Project: config.Project{Name: "gbme-path-test-" + filepath.Base(root)}, Parent: parent}).Validate()
		if err != nil || target != filepath.Join(homeDir, "gbme-path-test-"+filepath.Base(root)) {
			t.Fatalf("%s %v", target, err)
		}
	}
	link := filepath.Join(t.TempDir(), "parent")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	target, err := (State{Project: config.Project{Name: "api"}, Parent: link}).Validate()
	if err != nil || target != filepath.Join(root, "api") {
		t.Fatalf("%s %v", target, err)
	}
}

func TestConfigurationRulesAndPersistence(t *testing.T) {
	s := (State{Project: config.Project{Name: "first"}}).normalized()
	s.Project.Name = "second"
	if s.Configuration().Module != "second" {
		t.Fatal("inferred module did not follow name")
	}
	s.Project.Module = "example.com/team/custom"
	s.Project.Name = "third"
	if s.Configuration().Module != "example.com/team/custom" {
		t.Fatal("explicit module lost")
	}
	for _, db := range config.DatabaseChoices() {
		s.SelectDatabase(db.Value)
		if err := s.Configuration().Validate(); err != nil {
			t.Fatal(err)
		}
		if slices.Contains(s.Steps(), DataAccess) != (db.Value != "none") {
			t.Fatal(s.Steps())
		}
	}
	for _, p := range []config.Project{
		{Name: "api", Database: "mysql"}, {Name: "api", Database: "sqlite"},
		{Name: "api", Database: "postgres", DataAccess: "gorm"},
		{Name: "api", Database: "none", DataAccess: "sql"},
	} {
		m := NewModel(context.Background(), State{Project: p, Parent: t.TempDir()}, generator.Engine{})
		m.enter(Review)
		if cmd := press(m, tea.KeyEnter); cmd != nil || m.err == nil || m.phase != editing {
			t.Fatalf("accepted %+v", p)
		}
	}
}

func TestFinalSubmission(t *testing.T) {
	var calls int
	var submitted config.Project
	var parent string
	var verified bool
	m := testModel(t, generatorFunc(func(ctx context.Context, dir string, p config.Project, verify bool) (string, error) {
		calls++
		submitted = p
		parent = dir
		verified = verify
		return filepath.Join(dir, p.Name), nil
	}))
	typeText(m, "my-api")
	m.state.Project.Module = "example.com/team/api"
	m.state.Project.HTTP = "fiber"
	m.state.SelectDatabase("postgres")
	m.state.Project.Redis = true
	m.state.Project.Docker = true
	m.state.Verify = true
	reachReview(t, m)
	if calls != 0 {
		t.Fatal("generation began without confirmation")
	}
	cmd := press(m, tea.KeyEnter)
	if m.phase != generating || calls != 0 {
		t.Fatal("generation must run asynchronously")
	}
	if again := press(m, tea.KeyEnter); again != nil {
		t.Fatal("duplicate submission")
	}
	press(m, tea.KeyEscape)
	if m.phase != generating {
		t.Fatal("back during generation")
	}
	complete(t, m, cmd)
	if m.phase != succeeded || calls != 1 || parent != m.state.Parent || !verified || submitted != m.state.Configuration() {
		t.Fatalf("%d %+v", calls, submitted)
	}
	if !strings.Contains(m.body(), "docker compose up --build") || !strings.Contains(m.body(), m.dir) {
		t.Fatal(m.body())
	}
}

func TestErrorsAndCancellation(t *testing.T) {
	t.Run("before submission", func(t *testing.T) {
		m := testModel(t, generatorFunc(func(context.Context, string, config.Project, bool) (string, error) {
			t.Fatal("called generator after cancel")
			return "", nil
		}))
		cmd := ctrlC(m)
		if _, ok := cmd().(tea.QuitMsg); !ok || m.phase != canceled {
			t.Fatal("did not quit cleanly")
		}
	})
	t.Run("generation failure and retry", func(t *testing.T) {
		failure := errors.New("go test ./... failed: compiler diagnostic")
		calls := 0
		m := testModel(t, generatorFunc(func(_ context.Context, parent string, p config.Project, _ bool) (string, error) {
			calls++
			if calls == 1 {
				return "", failure
			}
			return filepath.Join(parent, p.Name), nil
		}))
		typeText(m, "api")
		reachReview(t, m)
		complete(t, m, press(m, tea.KeyEnter))
		if m.phase != failed || !errors.Is(m.err, failure) || strings.Contains(m.body(), "successfully") {
			t.Fatal(m.body())
		}
		press(m, tea.KeyEscape)
		if m.step != Review || m.state.Project.Name != "api" {
			t.Fatal("retry lost configuration")
		}
		complete(t, m, press(m, tea.KeyEnter))
		if m.phase != succeeded {
			t.Fatal(m.body())
		}
	})
	t.Run("wait for cleanup", func(t *testing.T) {
		started := make(chan struct{})
		release := make(chan struct{})
		messages := make(chan []tea.Msg, 1)
		m := testModel(t, generatorFunc(func(ctx context.Context, _ string, _ config.Project, _ bool) (string, error) {
			close(started)
			<-ctx.Done()
			<-release
			return "", fmt.Errorf("staged build: %w", ctx.Err())
		}))
		typeText(m, "api")
		reachReview(t, m)
		cmd := press(m, tea.KeyEnter)
		go func() { messages <- executeCommand(cmd) }()
		select {
		case <-started:
		case <-time.After(3 * time.Second):
			t.Fatal("generation not started")
		}
		if quit := ctrlC(m); quit != nil || m.phase != canceling {
			t.Fatal("quit before cleanup")
		}
		if quit := ctrlC(m); quit != nil {
			t.Fatal("repeated cancel quit early")
		}
		close(release)
		select {
		case msgs := <-messages:
			for _, msg := range msgs {
				if _, ok := msg.(generatedMsg); ok {
					_, cmd = m.Update(msg)
				}
			}
		case <-time.After(3 * time.Second):
			t.Fatal("cleanup did not finish")
		}
		if m.phase != canceled || cmd == nil {
			t.Fatal("did not quit after cleanup")
		}
	})
	t.Run("publication wins cancellation race", func(t *testing.T) {
		m := testModel(t, nil)
		m.phase = canceling
		m.Update(generatedMsg{dir: "/tmp/completed-api"})
		if m.phase != succeeded {
			t.Fatal("completed project reported canceled")
		}
	})
	t.Run("context ends", func(t *testing.T) {
		m := testModel(t, nil)
		_, cmd := m.Update(contextEndedMsg{})
		if cmd == nil || m.phase != canceled {
			t.Fatal("context cancellation ignored")
		}
	})
}

func TestDestinationRecheckedOnConfirmation(t *testing.T) {
	m := testModel(t, nil)
	typeText(m, "api")
	reachReview(t, m)
	if err := os.Mkdir(filepath.Join(m.state.Parent, "api"), 0700); err != nil {
		t.Fatal(err)
	}
	if cmd := press(m, tea.KeyEnter); cmd != nil || m.err == nil || m.phase != editing {
		t.Fatal("existing destination was submitted")
	}
}

func TestGeneratorIntegration(t *testing.T) {
	for _, http := range config.HTTPChoices() {
		for _, database := range config.DatabaseChoices() {
			for _, features := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/features=%t", http.Value, database.Value, features), func(t *testing.T) {
					m := testModel(t, nil)
					typeText(m, "api")
					m.state.Project.HTTP = http.Value
					m.state.SelectDatabase(database.Value)
					m.state.Project.Redis = features
					m.state.Project.Docker = features
					reachReview(t, m)
					complete(t, m, press(m, tea.KeyEnter))
					if m.phase != succeeded {
						t.Fatal(m.err)
					}
					manifest, err := generator.Load(m.dir)
					if err != nil || manifest.Project != m.state.Configuration() {
						t.Fatalf("%+v %v", manifest.Project, err)
					}
					for file, want := range map[string]bool{"Dockerfile": features, "compose.yaml": features, "internal/cache/cache.go": features, "internal/database/database.go": database.Value == "postgres"} {
						_, err := os.Stat(filepath.Join(m.dir, file))
						if (err == nil) != want {
							t.Fatalf("%s: %v", file, err)
						}
					}
				})
			}
		}
	}
}

func TestVerifiedGeneratorIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("generated project execution disabled")
	}
	m := testModel(t, nil)
	typeText(m, "verified-api")
	m.state.Verify = true
	reachReview(t, m)
	complete(t, m, press(m, tea.KeyEnter))
	if m.phase != succeeded {
		t.Fatal(m.err)
	}
	if _, err := generator.Load(m.dir); err != nil {
		t.Fatal(err)
	}
}

func TestVerificationFailureIntegration(t *testing.T) {
	tools := t.TempDir()
	if err := os.WriteFile(filepath.Join(tools, "go"), []byte("#!/bin/sh\nprintf 'dependency diagnostic\\n'\nexit 1\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", tools)
	m := testModel(t, nil)
	typeText(m, "failed-api")
	m.state.Verify = true
	reachReview(t, m)
	complete(t, m, press(m, tea.KeyEnter))
	if m.phase != failed || !strings.Contains(m.err.Error(), "dependency diagnostic") {
		t.Fatal(m.body())
	}
	entries, err := os.ReadDir(m.state.Parent)
	if err != nil || len(entries) != 0 {
		t.Fatalf("left partial output: %v %v", entries, err)
	}
}

func TestNextSteps(t *testing.T) {
	for _, db := range []string{"none", "postgres"} {
		for _, docker := range []bool{false, true} {
			for _, redis := range []bool{false, true} {
				s := State{Project: config.Project{Database: db, Docker: docker, Redis: redis}}
				text := strings.Join(NextSteps(s, "/tmp/a b's/api"), "\n")
				if !strings.Contains(text, `cd '/tmp/a b'"'"'s/api'`) {
					t.Fatal("unsafe cd", text)
				}
				if strings.Contains(text, "docker compose") != docker || strings.Contains(text, "make migrate-up") != (db == "postgres" && !docker) || strings.Contains(text, "# Start Redis") != (redis && !docker) {
					t.Fatal(text)
				}
				if strings.Contains(text, "set -a") == docker || !strings.Contains(text, "cp .env.example .env") {
					t.Fatal(text)
				}
			}
		}
	}
}

func TestNarrowAndPlainViews(t *testing.T) {
	m := testModel(t, nil)
	typeText(m, "my-api")
	m.noColor = true
	for _, width := range []int{20, 40, 80} {
		m.Update(tea.WindowSizeMsg{Width: width, Height: 24})
		for _, step := range m.state.Steps() {
			m.enter(step)
			view := m.View().Content
			if got := len(strings.Split(view, "\n")); got > 24 {
				t.Fatalf("width %d: view has %d lines in a 24-line terminal", width, got)
			}
			if strings.Contains(view, "\x1b") {
				t.Fatal("NO_COLOR emitted ANSI styles")
			}
			for _, line := range strings.Split(view, "\n") {
				if ansi.StringWidth(line) > width {
					t.Fatalf("width %d: %q", width, line)
				}
			}
		}
	}
	m.enter(Review)
	m.Update(tea.WindowSizeMsg{Width: 30, Height: 16})
	before := m.View().Content
	press(m, tea.KeyPgDown)
	if m.View().Content == before {
		t.Fatal("review cannot scroll")
	}
}
