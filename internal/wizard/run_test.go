package wizard

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"go-boiletplate-made-easy/internal/config"
)

func TestNonTerminalRunNeverGenerates(t *testing.T) {
	for _, terminal := range []string{"", "dumb", "xterm-256color"} {
		t.Run(terminal, func(t *testing.T) {
			t.Setenv("TERM", terminal)
			var out bytes.Buffer
			err := Run(context.Background(), strings.NewReader(""), &out, State{}, generatorFunc(func(context.Context, string, config.Project, bool) (string, error) {
				t.Fatal("non-terminal invoked generator")
				return "", nil
			}))
			if err == nil || !strings.Contains(err.Error(), "gbme new my-api --non-interactive") || out.Len() != 0 {
				t.Fatalf("output=%q error=%v", out.String(), err)
			}
		})
	}
}

func TestCharacterDeviceIsNotNecessarilyTerminal(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	file, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := TerminalError(file, file); err == nil {
		t.Fatal("accepted /dev/null as a terminal")
	}
}
