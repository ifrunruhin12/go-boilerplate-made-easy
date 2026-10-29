package config

import (
	"fmt"
	"go/token"
	"regexp"
	"strings"
)

var namePattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)
var identifierPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
var modulePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._~/-]*$`)

type Project struct {
	Name       string `json:"name"`
	Module     string `json:"module"`
	HTTP       string `json:"http"`
	Database   string `json:"database"`
	DataAccess string `json:"data_access"`
	Redis      bool   `json:"redis"`
	Docker     bool   `json:"docker"`
}

func (p *Project) Defaults() {
	if p.HTTP == "" {
		p.HTTP = "stdlib"
	}
	if p.Database == "" {
		p.Database = "none"
	}
	if p.DataAccess == "" {
		if choices := DataAccessChoices(p.Database); len(choices) > 0 {
			p.DataAccess = choices[0].Value
		}
	}
	if p.Module == "" {
		p.Module = p.Name
	}
}
func (p Project) Validate() error {
	if err := ValidateProjectName(p.Name); err != nil {
		return err
	}
	if !modulePattern.MatchString(p.Module) || strings.Contains(p.Module, "..") || strings.HasSuffix(p.Module, "/") || strings.Contains(p.Module, "//") || strings.Contains(p.Module, "/vendor/") {
		return fmt.Errorf("invalid Go module path %q", p.Module)
	}
	if !containsChoice(HTTPChoices(), p.HTTP) {
		return fmt.Errorf("unknown HTTP framework %q; choose stdlib, gin, or fiber", p.HTTP)
	}
	if !containsChoice(DatabaseChoices(), p.Database) {
		return fmt.Errorf("database %q is not implemented; choose none or postgres (SQLite, MySQL and MongoDB are planned)", p.Database)
	}
	if !containsChoice(DataAccessChoices(p.Database), p.DataAccess) {
		return fmt.Errorf("incompatible database/data-access pair %s/%s; use none/none or postgres/sql (GORM and sqlc are planned)", p.Database, p.DataAccess)
	}
	return nil
}

type Field struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func (f Field) GoName() string {
	parts := strings.Split(f.Name, "_")
	for i := range parts {
		parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
	}
	return strings.Join(parts, "")
}
func (f Field) SQLType() string {
	switch f.Type {
	case "string":
		return "TEXT"
	case "int", "int64":
		return "BIGINT"
	case "float64":
		return "DOUBLE PRECISION"
	case "bool":
		return "BOOLEAN"
	}
	return ""
}

type Resource struct {
	Name             string   `json:"name"`
	Fields           []Field  `json:"fields"`
	Methods          []string `json:"methods"`
	MigrationVersion int      `json:"migration_version,omitempty"`
	Persistent       bool     `json:"persistent"`
}

func (r Resource) Has(method string) bool {
	for _, m := range r.Methods {
		if m == method {
			return true
		}
	}
	return false
}
func (r Resource) Validate() error {
	if !identifierPattern.MatchString(r.Name) || strings.Contains(r.Name, "_") || len(r.Name) > 40 || token.Lookup(r.Name).IsKeyword() || r.Name == "health" || r.Name == "ready" || r.Name == "cache" || r.Name == "notes" || r.Name == "api" || r.Name == "server" || r.Name == "config" || r.Name == "database" || r.Name == "main" {
		return fmt.Errorf("resource name must be a lowercase Go identifier, without underscores or reserved route names")
	}
	if len(r.Fields) == 0 || len(r.Fields) > 32 {
		return fmt.Errorf("provide 1 to 32 fields, for example --fields name:string,email:string")
	}
	seen := map[string]bool{}
	for _, f := range r.Fields {
		if !identifierPattern.MatchString(f.Name) || strings.HasSuffix(f.Name, "_") || strings.Contains(f.Name, "__") || len(f.Name) > 40 || token.Lookup(f.Name).IsKeyword() || f.GoName() == "ID" || f.Name == "id" || f.GoName() == "Validate" || f.GoName() == "Record" {
			return fmt.Errorf("invalid or reserved field name %q", f.Name)
		}
		if seen[f.Name] {
			return fmt.Errorf("duplicate field %q", f.Name)
		}
		seen[f.Name] = true
		if f.SQLType() == "" {
			return fmt.Errorf("unsupported type %q; choose string, int, int64, float64 or bool", f.Type)
		}
	}
	if len(r.Methods) == 0 {
		return fmt.Errorf("select at least one HTTP method")
	}
	seen = map[string]bool{}
	for _, m := range r.Methods {
		switch m {
		case "GET", "POST", "PUT", "DELETE":
		default:
			return fmt.Errorf("method %q is unsupported; use GET,POST for endpoints or --crud for GET,POST,PUT,DELETE", m)
		}
		if seen[m] {
			return fmt.Errorf("duplicate method %q", m)
		}
		seen[m] = true
	}
	return nil
}
func ParseFields(s string) ([]Field, error) {
	var fields []Field
	for _, part := range strings.Split(s, ",") {
		pair := strings.Split(strings.TrimSpace(part), ":")
		if len(pair) != 2 {
			return nil, fmt.Errorf("invalid field %q; expected name:type", part)
		}
		fields = append(fields, Field{strings.TrimSpace(pair[0]), strings.TrimSpace(pair[1])})
	}
	return fields, nil
}

func (r Resource) HasType(typ string) bool {
	for _, f := range r.Fields {
		if f.Type == typ {
			return true
		}
	}
	return false
}
func (f Field) Sample() string {
	switch f.Type {
	case "string":
		if f.Name == "email" {
			return `"person@example.com"`
		}
		return `"example"`
	case "bool":
		return "true"
	case "float64":
		return "1.5"
	default:
		return "7"
	}
}
func (f Field) ZeroSample() string {
	if f.Type == "string" {
		if f.Name == "email" {
			return `"updated@example.com"`
		}
		return `"updated"`
	}
	if f.Type == "bool" {
		return "false"
	}
	return "0"
}
