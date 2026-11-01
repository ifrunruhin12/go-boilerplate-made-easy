package generator

import (
	"encoding/json"
	"fmt"
	"go-boiletplate-made-easy/internal/config"
)

// OpenAPISpec renders the OpenAPI 3.0.3 document for the generated API as JSON.
// JSON is used so no external YAML library is required. The result is embedded
// verbatim into the generated project and served at /openapi.json.
func OpenAPISpec(p config.Project, resources []config.Resource) ([]byte, error) {
	doc := openapiDocument{
		OpenAPI: "3.0.3",
		Info: openapiInfo{
			Title:   p.Name,
			Version: "1.0.0",
			Description: fmt.Sprintf(
				"Generated API for %s. HTTP: %s; database: %s/%s.",
				p.Name, p.HTTP, p.Database, p.DataAccess),
		},
		Servers: []openapiServer{{
			URL: "http://{host}:{port}",
			Variables: map[string]openapiVariable{
				"host": {Default: "0.0.0.0"},
				"port": {Default: "8080"},
			},
		}},
		Tags: []openapiTag{{Name: "generated"}},
	}

	schemas := map[string]any{}
	schemas["ApiError"] = map[string]any{
		"type": "object",
		"properties": map[string]any{
			"code":    map[string]any{"type": "string"},
			"message": map[string]any{"type": "string"},
		},
	}
	schemas["Health"] = map[string]any{
		"type":       "object",
		"properties": map[string]any{"status": map[string]any{"type": "string"}},
	}

	paths := map[string]map[string]openapiOperation{}
	paths["/health"] = map[string]openapiOperation{"get": opCreate("Liveness", 200, "Health")}
	paths["/ready"] = map[string]openapiOperation{"get": opCreate("Readiness", 200, "Health")}

	for _, r := range resources {
		name := schemaName(r)
		schemas[name] = recordSchema(r)
		schemas[name+"s"] = map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/" + name}}

		base := "/api/v1/" + r.Name
		ops := map[string]openapiOperation{}
		if r.Has("GET") {
			ops["get"] = opList("List "+r.Name, 200, name+"s")
		}
		if r.Has("POST") {
			ops["post"] = opCreate("Create "+r.Name, 201, name)
		}
		if len(ops) > 0 {
			paths[base] = ops
		}
		if r.Persistent {
			ids := map[string]openapiOperation{}
			idParam := map[string]any{"name": "id", "in": "path", "required": true, "schema": schemaOf("int64")}
			if r.Has("GET") {
				ids["get"] = opWithParams("Get "+r.Name, 200, name, []map[string]any{idParam})
			}
			if r.Has("PUT") {
				ids["put"] = opWithParams("Replace "+r.Name, 200, name, []map[string]any{idParam})
			}
			if r.Has("DELETE") {
				ids["delete"] = opWithParams("Delete "+r.Name, 204, "", []map[string]any{idParam})
			}
			if len(ids) > 0 {
				paths[base+"/{id}"] = ids
			}
		}
	}

	doc.Paths = paths
	doc.Components = map[string]map[string]any{"schemas": schemas}

	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return out, nil
}

type openapiDocument struct {
	OpenAPI    string                                 `json:"openapi"`
	Info       openapiInfo                            `json:"info"`
	Servers    []openapiServer                        `json:"servers"`
	Tags       []openapiTag                           `json:"tags"`
	Paths      map[string]map[string]openapiOperation `json:"paths"`
	Components map[string]map[string]any              `json:"components"`
}

type openapiInfo struct {
	Title       string `json:"title"`
	Version     string `json:"version"`
	Description string `json:"description"`
}

type openapiServer struct {
	URL       string                     `json:"url"`
	Variables map[string]openapiVariable `json:"variables"`
}

type openapiVariable struct {
	Default string `json:"default"`
}

type openapiTag struct {
	Name string `json:"name"`
}

type openapiOperation struct {
	Responses  map[string]any   `json:"responses,omitempty"`
	Summary    string           `json:"summary,omitempty"`
	Tags       []string         `json:"tags,omitempty"`
	Parameters []map[string]any `json:"parameters,omitempty"`
}

// schemaName returns the OpenAPI component name for a resource.
func schemaName(r config.Resource) string {
	if r.Name == "" {
		return r.Name
	}
	return r.Name[:1] + r.Name[1:]
}

// schemaOf maps a supported generator field type to a JSON schema.
func schemaOf(typ string) map[string]any {
	switch typ {
	case "int":
		return map[string]any{"type": "integer", "format": "int32"}
	case "int64":
		return map[string]any{"type": "integer", "format": "int64"}
	case "float64":
		return map[string]any{"type": "number", "format": "double"}
	case "bool":
		return map[string]any{"type": "boolean"}
	default:
		return map[string]any{"type": "string"}
	}
}

// recordSchema builds the named schema for a resource Record (with id).
func recordSchema(r config.Resource) map[string]any {
	props := map[string]any{"id": schemaOf("int64")}
	required := []string{"id"}
	for _, f := range r.Fields {
		s := schemaOf(f.Type)
		if f.Type == "string" {
			if f.Name == "email" {
				s["format"] = "email"
			} else {
				s["maxLength"] = 255
			}
		}
		props[f.Name] = s
		required = append(required, f.Name)
	}
	return map[string]any{"type": "object", "properties": props, "required": required}
}

// errorResponses builds the 4xx/5xx response entries shared by every operation.
func errorResponses() map[string]any {
	apiErrorRef := map[string]any{"$ref": "#/components/schemas/ApiError"}
	content := map[string]any{"application/json": map[string]any{"schema": apiErrorRef}}
	emptyContent := map[string]any{}
	return map[string]any{
		"400": map[string]any{"description": "Invalid input", "content": content},
		"404": map[string]any{"description": "Not found", "content": content},
		"500": map[string]any{"description": "Internal error", "content": content},
		"405": map[string]any{"description": "Method not allowed", "content": emptyContent},
	}
}

func successResponse(status int, dataRef string) map[string]any {
	data := map[string]any{"$ref": "#/components/schemas/" + dataRef}
	if dataRef == "" {
		data = map[string]any{}
	}
	responses := map[string]any{
		fmt.Sprintf("%d", status): map[string]any{
			"description": "Success",
			"content": map[string]any{
				"application/json": map[string]any{
					"schema": map[string]any{
						"type":       "object",
						"properties": map[string]any{"data": data, "error": map[string]any{"$ref": "#/components/schemas/ApiError"}},
					},
				},
			},
		},
	}
	for k, v := range errorResponses() {
		if _, exists := responses[k]; !exists {
			responses[k] = v
		}
	}
	return responses
}

// opWithParams builds an operation with the given path/query parameters.
func opWithParams(summary string, status int, dataRef string, params []map[string]any) openapiOperation {
	oper := openapiOperation{
		Responses: successResponse(status, dataRef),
		Summary:   summary,
		Tags:      []string{"generated"},
	}
	if len(params) > 0 {
		oper.Parameters = params
	}
	return oper
}

// opList builds a list operation with limit/offset query parameters.
func opList(summary string, status int, dataRef string) openapiOperation {
	return opWithParams(summary, status, dataRef, []map[string]any{
		{"name": "limit", "in": "query", "schema": schemaOf("int"), "description": "Page size, 1-100, default 20"},
		{"name": "offset", "in": "query", "schema": schemaOf("int"), "description": "Offset, default 0"},
	})
}

// opCreate builds a create operation with no path parameters.
func opCreate(summary string, status int, dataRef string) openapiOperation {
	return opWithParams(summary, status, dataRef, nil)
}
