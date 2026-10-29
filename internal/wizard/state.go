// Package wizard collects a project configuration and submits it to a generator.
// It does not render templates, manage dependencies, or publish files.
package wizard

import (
	"context"
	"go-boiletplate-made-easy/internal/config"
)

// Generator is implemented by generator.Engine. Tests may supply a controlled
// implementation to exercise errors and cancellation without a terminal.
type Generator interface {
	New(context.Context, string, config.Project, bool) (string, error)
}

type Step int

const (
	Name Step = iota
	Directory
	HTTP
	Database
	DataAccess
	Redis
	Docker
	Verify
	Review
)

type State struct {
	Project config.Project
	Parent  string
	Verify  bool
}

func (s State) normalized() State {
	// Keep an empty module as an inferred value, so editing the name also
	// updates the module. An explicitly supplied --module remains untouched.
	module := s.Project.Module
	s.Project.Defaults()
	s.Project.Module = module
	if s.Parent == "" {
		s.Parent = "."
	}
	return s
}

func (s State) Configuration() config.Project {
	p := s.Project
	p.Defaults()
	return p
}

func (s State) Steps() []Step {
	steps := []Step{Name, Directory, HTTP, Database}
	if s.Project.Database != "none" {
		steps = append(steps, DataAccess)
	}
	return append(steps, Redis, Docker, Verify, Review)
}

func (s *State) SelectDatabase(database string) {
	s.Project.Database = database
	choices := config.DataAccessChoices(database)
	for _, c := range choices {
		if c.Value == s.Project.DataAccess {
			return
		}
	}
	s.Project.DataAccess = ""
	if len(choices) > 0 {
		s.Project.DataAccess = choices[0].Value
	}
}

// Validate resolves the destination without creating it. Engine.New performs
// authoritative validation again at submission, including exclusive creation.
func (s State) Validate() (string, error) {
	if err := s.Configuration().Validate(); err != nil {
		return "", err
	}
	return resolveDestination(s.Parent, s.Project.Name)
}
