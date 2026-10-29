package wizard

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

func resolveDestination(parent, name string) (string, error) {
	if strings.TrimSpace(parent) == "" {
		return "", fmt.Errorf("enter an existing parent directory, for example .")
	}
	if strings.ContainsFunc(parent, unicode.IsControl) {
		return "", fmt.Errorf("directory paths cannot contain control characters")
	}
	if parent == "~" || strings.HasPrefix(parent, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		parent = filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(parent, "~"), "/"))
	} else if strings.HasPrefix(parent, "~") {
		return "", fmt.Errorf("use ~/ or an absolute path; ~user paths are not supported")
	}
	abs, err := filepath.Abs(parent)
	if err != nil {
		return "", fmt.Errorf("resolve output directory: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("open parent directory: %w; choose an existing directory", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is a file; choose an existing directory", abs)
	}
	abs, err = filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("resolve parent directory: %w", err)
	}
	target := filepath.Join(abs, name)
	if _, err := os.Lstat(target); err == nil {
		return "", fmt.Errorf("%s already exists; choose another name or parent directory", target)
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("inspect destination: %w", err)
	}
	return target, nil
}
