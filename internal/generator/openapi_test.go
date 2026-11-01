package generator

import (
	"encoding/json"
	"go-boiletplate-made-easy/internal/config"
	"strings"
	"testing"
)

func TestOpenAPISpec(t *testing.T) {
	p := config.Project{Name: "api", HTTP: "fiber", Database: "postgres", DataAccess: "sql"}
	p.Defaults()
	p.OpenAPI = true
	r := testResource(true)
	r.Methods = []string{"GET", "POST", "PUT", "DELETE"}

	body, err := OpenAPISpec(p, []config.Resource{r})
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	for _, key := range []string{"openapi", "info", "paths", "components"} {
		if _, ok := doc[key]; !ok {
			t.Fatalf("missing top-level key %s", key)
		}
	}
	if doc["openapi"] != "3.0.3" {
		t.Fatalf("openapi version: %v", doc["openapi"])
	}
	paths := doc["paths"].(map[string]any)
	for _, p := range []string{"/health", "/ready", "/api/v1/products", "/api/v1/products/{id}"} {
		if _, ok := paths[p]; !ok {
			t.Fatalf("missing path %s", p)
		}
	}
	// products should have all four CRUD methods split across base and id path.
	baseOps := paths["/api/v1/products"].(map[string]any)
	if _, ok := baseOps["get"]; !ok {
		t.Fatal("products missing list op")
	}
	if _, ok := baseOps["post"]; !ok {
		t.Fatal("products missing create op")
	}
	idOps := paths["/api/v1/products/{id}"].(map[string]any)
	for _, m := range []string{"get", "put", "delete"} {
		if _, ok := idOps[m]; !ok {
			t.Fatalf("products/{id} missing %s op", m)
		}
	}
	// Components should contain the record schema and a plural variant.
	schemas := doc["components"].(map[string]any)["schemas"].(map[string]any)
	for _, name := range []string{"products", "productss", "ApiError", "Health"} {
		if _, ok := schemas[name]; !ok {
			t.Fatalf("missing schema %s", name)
		}
	}
	// The schema should include id + the four fields.
	prod := schemas["products"].(map[string]any)
	props := prod["properties"].(map[string]any)
	for _, f := range []string{"id", "name", "price", "quantity", "active"} {
		if _, ok := props[f]; !ok {
			t.Fatalf("Products schema missing %s", f)
		}
	}
}

func TestOpenAPISpecEndpointMethodsOnly(t *testing.T) {
	p := config.Project{Name: "api"}
	p.Defaults()
	p.OpenAPI = true
	r := testResource(false)
	r.Methods = []string{"GET", "POST"}
	body, err := OpenAPISpec(p, []config.Resource{r})
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	json.Unmarshal(body, &doc)
	paths := doc["paths"].(map[string]any)
	base := paths["/api/v1/products"].(map[string]any)
	for _, m := range []string{"get", "post"} {
		if _, ok := base[m]; !ok {
			t.Fatalf("missing %s op", m)
		}
	}
	for _, m := range []string{"put", "delete"} {
		if _, ok := base[m]; ok {
			t.Fatalf("unexpected %s op on in-memory resource", m)
		}
	}
	// No /{id} path because the resource is not persistent.
	if _, ok := paths["/api/v1/products/{id}"]; ok {
		t.Fatal("persistent-only /{id} path present on in-memory resource")
	}
}

func TestOpenAPISpecFieldTypes(t *testing.T) {
	p := config.Project{Name: "api"}
	p.Defaults()
	p.OpenAPI = true
	r := config.Resource{
		Name: "widgets",
		Fields: []config.Field{
			{Name: "label", Type: "string"},
			{Name: "count", Type: "int"},
			{Name: "weight", Type: "float64"},
			{Name: "on", Type: "bool"},
			{Name: "email", Type: "string"},
		},
		Methods: []string{"GET", "POST"},
	}
	body, err := OpenAPISpec(p, []config.Resource{r})
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	json.Unmarshal(body, &doc)
	schemas := doc["components"].(map[string]any)["schemas"].(map[string]any)
	widgets := schemas["widgets"].(map[string]any)
	props := widgets["properties"].(map[string]any)
	if props["count"].(map[string]any)["type"] != "integer" || props["count"].(map[string]any)["format"] != "int32" {
		t.Fatalf("int mapping wrong: %v", props["count"])
	}
	if props["weight"].(map[string]any)["type"] != "number" {
		t.Fatalf("float64 mapping wrong: %v", props["weight"])
	}
	if props["on"].(map[string]any)["type"] != "boolean" {
		t.Fatalf("bool mapping wrong: %v", props["on"])
	}
	if props["email"].(map[string]any)["format"] != "email" {
		t.Fatalf("email mapping wrong: %v", props["email"])
	}
	// A non-email string should carry maxLength 255.
	if props["label"].(map[string]any)["maxLength"] != float64(255) {
		t.Fatalf("label maxLength wrong: %v", props["label"])
	}
	// An in-memory resource produces no /{id} path.
	paths := doc["paths"].(map[string]any)
	if _, has := paths["/api/v1/widgets/{id}"]; has {
		t.Fatal("in-memory resource should not expose /{id} path")
	}
	// Make sure there is no panic on empty resource set.
	if s, err := OpenAPISpec(p, nil); err != nil || !strings.Contains(string(s), `"openapi"`) {
		t.Fatalf("empty resources: %v", err)
	}
}
