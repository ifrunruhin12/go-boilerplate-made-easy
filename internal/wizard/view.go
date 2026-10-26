package wizard

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	mutedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
)

func (m *Model) contentWidth() int { return max(1, min(76, m.width-2)) }

func enabled(value bool) string {
	if value {
		return "Enabled"
	}
	return "Disabled"
}

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }

// NextSteps follows the generated README. Native apps read exported variables,
// while Compose provisions the selected services and runs migrations itself.
func NextSteps(s State, dir string) []string {
	commands := []string{"cd " + shellQuote(dir)}
	if s.Project.Docker {
		return append(commands, "cp .env.example .env", "docker compose up --build")
	}
	if !s.Verify {
		commands = append(commands, "go mod tidy")
	}
	commands = append(commands, "cp .env.example .env")
	if s.Project.Database != "none" || s.Project.Redis {
		commands = append(commands, "# Edit .env to match your running services.")
	}
	commands = append(commands, "set -a; . ./.env; set +a")
	if s.Project.Database == "postgres" {
		commands = append(commands, "# Start PostgreSQL with the database/user in DATABASE_URL.", "make migrate-up")
	}
	if s.Project.Redis {
		commands = append(commands, "# Start Redis at REDIS_URL.")
	}
	return append(commands, "go run ./cmd/server")
}

func (m *Model) summary() string {
	p := m.state.Configuration()
	dir := m.dir
	if dir == "" {
		dir = filepath.Join(m.state.Parent, p.Name)
	}
	return fmt.Sprintf("Project:     %s\nOutput:      %s\nModule:      %s\nHTTP:        %s\nDatabase:    %s\nData access: %s\nRedis:       %s\nDocker:      %s\nVerify:      %s", p.Name, dir, p.Module, p.HTTP, p.Database, p.DataAccess, enabled(p.Redis), enabled(p.Docker), enabled(m.state.Verify))
}

func (m *Model) body() string {
	switch m.phase {
	case generating, canceling:
		if m.phase == canceling {
			return m.spinner.View() + " Canceling; waiting for generator cleanup..."
		}
		if m.state.Verify {
			return m.spinner.View() + " Generating and verifying project...\n\nResolving dependencies, running tests and building in a staging directory. This may need network access."
		}
		return m.spinner.View() + " Generating project...\n\nRendering and formatting selected templates, then publishing files."
	case canceled:
		return "Canceled."
	case failed:
		return errorStyle.Render("Generation failed") + "\n\n" + ansi.Strip(m.err.Error()) + "\n\nThe generator did not report a completed project. Review the error before retrying."
	case succeeded:
		body := titleStyle.Render("Project generated successfully!") + "\n\n" + m.summary()
		if !m.state.Verify {
			body += "\n\nSource formatted; dependency, test and build verification was skipped."
		}
		return body + "\n\nNext steps:\n  " + strings.Join(NextSteps(m.state, m.dir), "\n  ")
	}
	var title, description string
	switch m.step {
	case Name:
		title, description = "Name your project", "Lowercase kebab-case, up to 64 characters. Example: billing-api."
	case Directory:
		title, description = "Choose the output directory", "Enter an existing parent directory. A new folder named "+m.state.Project.Name+" will be created inside it."
	case HTTP:
		title, description = "Choose your HTTP implementation", "Each option includes routing, request validation, health checks and shutdown."
	case Database:
		title, description = "Choose your database", "Only implemented database options are shown."
	case DataAccess:
		title, description = "Choose PostgreSQL data access", "SQL is the supported strategy for PostgreSQL."
	case Redis:
		title, description = "Include Redis?", "Adds a Redis client, readiness check and cache example. Configure REDIS_URL in the generated .env; Compose connects its Redis service automatically."
	case Docker:
		title, description = "Include Docker?", "Adds a Dockerfile and Compose configuration for the app and selected services. PostgreSQL migrations run before app startup."
	case Verify:
		title, description = "Verify before publishing?", "Run go mod tidy, generated tests and a build in staging. Requires Go and dependency access; failed verification removes staged output."
	case Review:
		body := titleStyle.Render("Review your project") + "\n\n" + m.summary() + "\n\nIncludes config, logging, validation, CORS, health/readiness and graceful shutdown. Existing destinations are refused."
		if m.state.Project.HTTP == "gin" {
			body += "\n\nGin is a preview; executable validation remains pending."
		}
		return body + "\n\nPress Enter to generate, or Esc to revise."
	}
	body := titleStyle.Render(title) + "\n\n" + description + "\n\n"
	if m.step == Name || m.step == Directory {
		return body + m.input.View()
	}
	choices, selected := m.choices()
	for _, c := range choices {
		line := "  ( ) " + c.Label
		if c.Value == selected {
			line = selectedStyle.Render("> (*) " + c.Label)
		}
		body += line + "\n      " + c.Description + "\n\n"
	}
	if m.step >= Redis && m.step <= Verify {
		value := m.state.Project.Redis
		if m.step == Docker {
			value = m.state.Project.Docker
		}
		if m.step == Verify {
			value = m.state.Verify
		}
		mark := "[ ]"
		if value {
			mark = "[x]"
		}
		body += selectedStyle.Render(mark+" "+enabled(value)) + "\n\nSpace to toggle; Enter to continue."
	}
	return strings.TrimRight(body, "\n")
}

func (m *Model) header() string {
	header := titleStyle.Render("GBME") + "  Go Boilerplate Made Easy"
	if m.phase == editing {
		header += fmt.Sprintf("\nStep %d of %d", m.stepIndex()+1, len(m.state.Steps()))
	}
	return header
}

func (m *Model) footer() string {
	if m.phase == generating || m.phase == canceling {
		return "Ctrl+C cancel"
	}
	if m.phase == succeeded {
		return "Enter exit | Up/Down scroll"
	}
	if m.phase == failed {
		return "Esc review | Enter exit | Up/Down scroll"
	}
	if m.phase == canceled {
		return ""
	}
	if m.step == Name || m.step == Directory {
		return "Enter next | Esc back | PgUp/PgDn scroll | Ctrl+C exit"
	}
	if m.step == Review {
		return "Enter generate | Esc back | Up/Down scroll | Ctrl+C exit"
	}
	return "Arrows select | Space toggle | Enter next | Esc/Backspace back | PgUp/PgDn scroll | Ctrl+C exit"
}

func (m *Model) wrapped(s string) string {
	if m.noColor {
		s = ansi.Strip(s)
	}
	return ansi.Wrap(s, m.contentWidth(), "")
}

func (m *Model) refreshViewport() {
	m.viewport.SetWidth(m.contentWidth())
	header, footer := m.wrapped(m.header()), m.wrapped(m.footer())
	h := max(1, m.height-lipgloss.Height(header)-lipgloss.Height(footer)-4)
	if m.err != nil && m.phase == editing {
		h -= lipgloss.Height(m.wrapped(m.err.Error())) + 1
	}
	m.viewport.SetHeight(max(1, h))
	m.viewport.SetContent(m.wrapped(m.body()))
}

func (m *Model) View() tea.View {
	// Render from a copy: View must not change navigation or scroll state.
	rendered := *m
	rendered.refreshViewport()
	body := rendered.viewport.View()
	if m.err != nil && m.phase == editing {
		body += "\n\n" + m.wrapped(errorStyle.Render(m.err.Error()))
	}
	content := m.wrapped(m.header()) + "\n\n" + body + "\n\n" + m.wrapped(mutedStyle.Render(m.footer()))
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}
