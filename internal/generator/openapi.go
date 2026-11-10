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
		"type":     "object",
		"required": []string{"code", "message"},
		"properties": map[string]any{
			"code":    map[string]any{"type": "string"},
			"message": map[string]any{"type": "string"},
		},
	}
	schemas["ErrorEnvelope"] = map[string]any{
		"type":       "object",
		"required":   []string{"error"},
		"properties": map[string]any{"error": map[string]any{"$ref": "#/components/schemas/ApiError"}},
	}
	schemas["Health"] = map[string]any{
		"type":       "object",
		"properties": map[string]any{"status": map[string]any{"type": "string"}},
	}

	paths := map[string]map[string]openapiOperation{}
	paths["/health"] = map[string]openapiOperation{"get": opCreate("Liveness", 200, "Health")}
	paths["/ready"] = map[string]openapiOperation{"get": opCreate("Readiness", 200, "Health")}
	paths["/ready"]["get"].Responses["503"] = errorResponse("Dependencies unavailable")

	for _, r := range resources {
		name := schemaName(r)
		schemas[name] = recordSchema(r)
		schemas[name+"Input"] = inputSchema(r)
		schemas[name+"s"] = map[string]any{"type": "array", "items": map[string]any{"$ref": "#/components/schemas/" + name}}

		base := "/api/v1/" + r.Name
		ops := map[string]openapiOperation{}
		if r.Has("GET") {
			ops["get"] = opList("List "+r.Name, 200, name+"s")
		}
		if r.Has("POST") {
			ops["post"] = opInput("Create "+r.Name, 201, name, nil)
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
				ids["put"] = opInput("Replace "+r.Name, 200, name, []map[string]any{idParam})
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
	Responses   map[string]any   `json:"responses,omitempty"`
	Summary     string           `json:"summary,omitempty"`
	Tags        []string         `json:"tags,omitempty"`
	Parameters  []map[string]any `json:"parameters,omitempty"`
	RequestBody map[string]any   `json:"requestBody,omitempty"`
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
	schema := inputSchema(r)
	schema["properties"].(map[string]any)["id"] = schemaOf("int64")
	schema["required"] = append([]string{"id"}, schema["required"].([]string)...)
	return schema
}

// inputSchema matches the strict JSON Input accepted by create/update handlers.
// IDs are assigned by storage or supplied in the path, never in the JSON body.
func inputSchema(r config.Resource) map[string]any {
	props := map[string]any{}
	required := []string{}
	for _, f := range r.Fields {
		s := schemaOf(f.Type)
		if f.Type == "string" {
			s["minLength"] = 1
			s["maxLength"] = 255
			if f.Name == "email" {
				s["format"] = "email"
			}
		}
		props[f.Name] = s
		required = append(required, f.Name)
	}
	return map[string]any{"type": "object", "properties": props, "required": required, "additionalProperties": false}
}

func errorResponse(description string) map[string]any {
	return map[string]any{
		"description": description,
		"content": map[string]any{"application/json": map[string]any{
			"schema": map[string]any{"$ref": "#/components/schemas/ErrorEnvelope"},
		}},
	}
}

// errorResponses builds the 4xx/5xx response entries shared by every operation.
func errorResponses() map[string]any {
	return map[string]any{
		"400": errorResponse("Invalid input"),
		"403": errorResponse("Origin is not allowed"),
		"404": errorResponse("Not found"),
		"405": errorResponse("Method not allowed"),
		"500": errorResponse("Internal error"),
	}
}

func successResponse(status int, dataRef string) map[string]any {
	response := map[string]any{"description": "Success"}
	if status != 204 && dataRef != "" {
		response["content"] = map[string]any{"application/json": map[string]any{
			"schema": map[string]any{
				"type":       "object",
				"required":   []string{"data"},
				"properties": map[string]any{"data": map[string]any{"$ref": "#/components/schemas/" + dataRef}},
			},
		}}
	}
	responses := errorResponses()
	responses[fmt.Sprintf("%d", status)] = response
	return responses
}

func opInput(summary string, status int, schema string, params []map[string]any) openapiOperation {
	op := opWithParams(summary, status, schema, params)
	op.RequestBody = map[string]any{
		"required": true,
		"content": map[string]any{"application/json": map[string]any{
			"schema": map[string]any{"$ref": "#/components/schemas/" + schema + "Input"},
		}},
	}
	return op
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
