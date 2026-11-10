package config

import "fmt"

// Choice describes an implemented generator capability. CLI interfaces use this
// catalog; compatibility is validated here rather than in individual screens.
type Choice struct {
	Value, Label, Description string
}

func HTTPChoices() []Choice {
	return []Choice{
		{"stdlib", "Standard library", "net/http routing, middleware and graceful shutdown."},
		{"gin", "Gin (preview)", "Gin handlers and middleware; full executable validation is pending."},
		{"fiber", "Fiber", "Fiber v2 routing, middleware and graceful shutdown."},
	}
}

func DatabaseChoices() []Choice {
	return []Choice{
		{"none", "None", "HTTP-only backend; endpoints can use in-memory storage."},
		{"postgres", "PostgreSQL", "Connection pooling, SQL migrations and a transaction example."},
	}
}

func DataAccessChoices(database string) []Choice {
	switch database {
	case "none":
		return []Choice{{"none", "None", "No database client or migrations."}}
	case "postgres":
		return []Choice{{"sql", "SQL with pgx", "database/sql with the pgx driver. GORM and sqlc are not implemented."}}
	default:
		return nil
	}
}

func containsChoice(choices []Choice, value string) bool {
	for _, choice := range choices {
		if choice.Value == value {
			return true
		}
	}
	return false
}

func ValidateProjectName(name string) error {
	if !namePattern.MatchString(name) || len(name) > 64 {
		return fmt.Errorf("project name must be lowercase kebab-case, start with a letter, and contain at most 64 characters")
	}
	return nil
}
