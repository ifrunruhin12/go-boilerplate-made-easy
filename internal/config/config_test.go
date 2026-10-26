package config

import (
	"testing"
)

func TestProjectChoices(t *testing.T) {
	for _, http := range []string{"stdlib", "gin", "fiber"} {
		for _, db := range []string{"none", "postgres"} {
			p := Project{Name: "my-api", HTTP: http, Database: db}
			p.Defaults()
			if err := p.Validate(); err != nil {
				t.Fatal(err)
			}
			if p.Module != "my-api" {
				t.Fatal(p.Module)
			}
		}
	}
}
func TestInvalidProjects(t *testing.T) {
	for _, p := range []Project{{Name: "../outside"}, {Name: "My API"}, {Name: "-api"}, {Name: "a/b"}, {Name: "api", HTTP: "echo"}, {Name: "api", Database: "sqlite"}, {Name: "api", Database: "postgres", DataAccess: "gorm"}, {Name: "api", DataAccess: "sql"}, {Name: "api", Module: "example.com/a\nmalicious"}, {Name: "api", Module: "example.com/../escape"}} {
		p.Defaults()
		if p.Validate() == nil {
			t.Fatalf("accepted %#v", p)
		}
	}
}
func TestResourceValidation(t *testing.T) {
	for _, tc := range []struct{ name, fields, method string }{{"users", "name:string,email:string", "GET"}, {"products", "price:float64,quantity:int64,active:bool", "POST"}} {
		fields, err := ParseFields(tc.fields)
		if err != nil {
			t.Fatal(err)
		}
		if err = (Resource{Name: tc.name, Fields: fields, Methods: []string{tc.method}}).Validate(); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct{ name, fields, method string }{{"health", "name:string", "GET"}, {"../escape", "name:string", "GET"}, {"users", "id:int64", "GET"}, {"users", "i_d:string", "GET"}, {"users", "name:object", "GET"}, {"users", "name:string,name:string", "GET"}, {"users", "name:string", "PATCH"}, {"users", "name:string", "GET,GET"}, {"users", "validate:string", "GET"}} {
		fields, err := ParseFields(tc.fields)
		if err == nil {
			methods := []string{tc.method}
			if tc.method == "GET,GET" {
				methods = []string{"GET", "GET"}
			}
			err = (Resource{Name: tc.name, Fields: fields, Methods: methods}).Validate()
		}
		if err == nil {
			t.Fatalf("accepted %#v", tc)
		}
	}
	if _, err := ParseFields("broken"); err == nil {
		t.Fatal("malformed field")
	}
}
