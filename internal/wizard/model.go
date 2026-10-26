package wizard

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"go-boiletplate-made-easy/internal/config"
)

type phase int

const (
	editing phase = iota
	generating
	canceling
	succeeded
	failed
	canceled
)

type generatedMsg struct {
	dir string
	err error
}
type contextEndedMsg struct{}

type Model struct {
	state    State
	step     Step
	phase    phase
	input    textinput.Model
	spinner  spinner.Model
	viewport viewport.Model
	width    int
	height   int
	err      error
	dir      string
	ctx      context.Context
	cancel   context.CancelFunc
	done     chan struct{}
	engine   Generator
	noColor  bool
}

func NewModel(ctx context.Context, state State, engine Generator) *Model {
	m := &Model{
		state: state.normalized(), ctx: ctx, engine: engine,
		width: 80, height: 30, spinner: spinner.New(),
		viewport: viewport.New(),
	}
	m.enter(Name)
	return m
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.input.Focus(), func() tea.Msg {
		<-m.ctx.Done()
		return contextEndedMsg{}
	})
}

func (m *Model) enter(step Step) tea.Cmd {
	m.step, m.err = step, nil
	m.viewport.GotoTop()
	m.input = textinput.New()
	m.input.SetWidth(max(1, m.contentWidth()-2))
	styles := m.input.Styles()
	styles.Cursor.Blink = false
	m.input.SetStyles(styles)
	switch step {
	case Name:
		m.input.Placeholder = "my-api"
		m.input.CharLimit = 128
		m.input.SetValue(m.state.Project.Name)
	case Directory:
		m.input.Placeholder = "."
		m.input.CharLimit = 4096
		m.input.SetValue(m.state.Parent)
	default:
		return nil
	}
	m.input.CursorEnd()
	return m.input.Focus()
}

func (m *Model) stepIndex() int {
	for i, step := range m.state.Steps() {
		if step == m.step {
			return i
		}
	}
	return 0
}

func (m *Model) backward() tea.Cmd {
	if i := m.stepIndex(); i > 0 {
		return m.enter(m.state.Steps()[i-1])
	}
	return nil
}

func (m *Model) forward() tea.Cmd {
	m.err = nil
	switch m.step {
	case Name:
		m.err = config.ValidateProjectName(m.state.Project.Name)
	case Directory:
		var dir string
		dir, m.err = resolveDestination(m.state.Parent, m.state.Project.Name)
		if m.err == nil {
			m.state.Parent = filepath.Dir(dir)
		}
	case HTTP, Database, DataAccess:
		choices, selected := m.choices()
		valid := false
		for _, c := range choices {
			valid = valid || c.Value == selected
		}
		if !valid {
			m.err = fmt.Errorf("use the arrow keys to select one of the supported options")
		}
	}
	if m.err != nil {
		return nil
	}
	if m.step == Review {
		return m.submit()
	}
	return m.enter(m.state.Steps()[m.stepIndex()+1])
}

func (m *Model) submit() tea.Cmd {
	_, m.err = m.state.Validate()
	if m.err != nil {
		return nil
	}
	// Capture immutable submission values. All rendering and filesystem work is
	// performed by Engine.New in this command, outside the event loop.
	state, engine := m.state, m.engine
	ctx, cancel := context.WithTimeout(m.ctx, 5*time.Minute)
	m.cancel = cancel
	done := make(chan struct{})
	m.done = done
	m.phase = generating
	m.viewport.GotoTop()
	return tea.Batch(m.spinner.Tick, func() tea.Msg {
		defer close(done)
		defer cancel()
		dir, err := engine.New(ctx, state.Parent, state.Configuration(), state.Verify)
		if err != nil && ctx.Err() != nil {
			err = errors.Join(ctx.Err(), err)
		}
		return generatedMsg{dir: dir, err: err}
	})
}

func (m *Model) cancelWork() tea.Cmd {
	if m.phase == generating || m.phase == canceling {
		m.phase = canceling
		m.cancel()
		// Wait for Engine.New to finish cleanup before quitting the program.
		return nil
	}
	if m.phase == editing {
		m.phase = canceled
	}
	return tea.Quit
}

func (m *Model) choices() ([]config.Choice, string) {
	switch m.step {
	case HTTP:
		return config.HTTPChoices(), m.state.Project.HTTP
	case Database:
		return config.DatabaseChoices(), m.state.Project.Database
	case DataAccess:
		return config.DataAccessChoices(m.state.Project.Database), m.state.Project.DataAccess
	}
	return nil, ""
}

func (m *Model) choose(delta int) {
	choices, selected := m.choices()
	if len(choices) == 0 {
		return
	}
	i := 0
	for j, choice := range choices {
		if choice.Value == selected {
			i = j
		}
	}
	value := choices[(i+delta+len(choices))%len(choices)].Value
	switch m.step {
	case HTTP:
		m.state.Project.HTTP = value
	case Database:
		m.state.SelectDatabase(value)
	case DataAccess:
		m.state.Project.DataAccess = value
	}
	m.err = nil
}

func (m *Model) toggle() {
	switch m.step {
	case Redis:
		m.state.Project.Redis = !m.state.Project.Redis
	case Docker:
		m.state.Project.Docker = !m.state.Project.Docker
	case Verify:
		m.state.Verify = !m.state.Verify
	}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	defer func() {
		m.refreshViewport()
		// On small terminals, keep the active input/selection visible. Paging
		// remains available to read descriptions above or below it.
		if m.phase != editing || m.step == Review {
			return
		}
		key, isKey := msg.(tea.KeyPressMsg)
		_, isResize := msg.(tea.WindowSizeMsg)
		if (!isKey && !isResize) || key.String() == "pgup" || key.String() == "pgdown" {
			return
		}
		lines := strings.Split(ansi.Strip(m.wrapped(m.body())), "\n")
		for i, line := range lines {
			if (m.step == Name || m.step == Directory) && i == len(lines)-1 ||
				strings.Contains(line, "(*)") || strings.Contains(line, "[x]") || strings.Contains(line, "[ ]") {
				m.viewport.EnsureVisible(i, 0, 0)
				break
			}
		}
	}()
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = max(1, msg.Width), max(1, msg.Height)
		m.input.SetWidth(max(1, m.contentWidth()-2))
		m.refreshViewport()
		return m, nil
	case generatedMsg:
		if m.phase != generating && m.phase != canceling {
			return m, nil
		}
		m.dir, m.err = msg.dir, msg.err
		m.viewport.GotoTop()
		if msg.err == nil {
			// Publication can finish just before cancellation; report its actual
			// outcome rather than telling the user that no project was created.
			m.phase = succeeded
		} else if m.phase == canceling && errors.Is(msg.err, context.Canceled) {
			m.phase = canceled
			return m, tea.Quit
		} else {
			m.phase = failed
		}
		return m, nil
	case contextEndedMsg:
		return m, m.cancelWork()
	case tea.KeyPressMsg:
		key := msg.String()
		if key == "ctrl+c" {
			return m, m.cancelWork()
		}
		if m.phase == generating || m.phase == canceling {
			return m, nil
		}
		if key == "pgup" || key == "pgdown" {
			m.refreshViewport()
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
		if m.phase == succeeded || m.phase == failed {
			if key == "enter" || key == "q" {
				return m, tea.Quit
			}
			if m.phase == failed && (key == "esc" || key == "backspace") {
				m.phase = editing
				return m, m.enter(Review)
			}
			m.refreshViewport()
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
		if key == "esc" || (key == "backspace" && m.step != Name && m.step != Directory) {
			return m, m.backward()
		}
		if key == "enter" {
			return m, m.forward()
		}
		if m.step == Review {
			m.refreshViewport()
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
		if key == "up" || key == "left" || key == "down" || key == "right" {
			if m.step >= Redis && m.step <= Verify {
				m.toggle()
			} else if key == "up" || key == "left" {
				m.choose(-1)
			} else {
				m.choose(1)
			}
		}
		if key == "space" {
			m.toggle()
		}
	}
	if m.phase == generating || m.phase == canceling {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	if m.phase == editing && (m.step == Name || m.step == Directory) {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		if m.step == Name {
			m.state.Project.Name = m.input.Value()
		} else {
			m.state.Parent = m.input.Value()
		}
		return m, cmd
	}
	return m, nil
}
