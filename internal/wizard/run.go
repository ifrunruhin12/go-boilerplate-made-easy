package wizard

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/term"
)

// TerminalError prevents terminal reads in CI, pipes, files and dumb terminals.
// Check both streams: a terminal stdin with redirected stdout is not a TUI.
func TerminalError(in io.Reader, out io.Writer) error {
	input, inputOK := in.(interface{ Fd() uintptr })
	output, outputOK := out.(interface{ Fd() uintptr })
	if os.Getenv("CI") != "" || os.Getenv("TERM") == "dumb" || os.Getenv("TERM") == "" ||
		!inputOK || !outputOK || !term.IsTerminal(input.Fd()) || !term.IsTerminal(output.Fd()) {
		return fmt.Errorf("the wizard needs an interactive terminal for input and output (TERM must be set, not dumb; CI must be unset); use gbme new my-api --non-interactive [flags], or gbme --help")
	}
	return nil
}

func Run(ctx context.Context, in io.Reader, out io.Writer, state State, engine Generator) error {
	if err := TerminalError(in, out); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	m := NewModel(ctx, state, engine)
	_, m.noColor = os.LookupEnv("NO_COLOR")
	// Bubble Tea also detects terminal color capabilities and honors NO_COLOR.
	program := tea.NewProgram(m, tea.WithInput(in), tea.WithOutput(out))
	defer func() {
		if m.cancel != nil {
			m.cancel()
		}
		if m.done != nil {
			<-m.done
		}
	}()
	_, err := program.Run()
	if err != nil {
		return fmt.Errorf("terminal UI: %w; try gbme new my-api --non-interactive", err)
	}
	if m.phase == succeeded {
		_, err = fmt.Fprintln(out, strings.TrimSpace(ansi.Strip(m.body())))
		return err
	}
	if m.phase == failed {
		return m.err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	_, err = fmt.Fprintln(out, "Canceled.")
	return err
}
