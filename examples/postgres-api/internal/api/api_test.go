package api

import (
	"strings"
	"testing"
)

func TestDecode(t *testing.T) {
	for _, body := range []string{`null`, `[]`, `{"name":"x","unknown":1}`, `{"name":"x"}{}`, `{"name":`} {
		var dst struct {
			Name string `json:"name"`
		}
		if Decode(strings.NewReader(body), &dst) == nil {
			t.Fatalf("accepted %s", body)
		}
	}
	var dst struct {
		Name string `json:"name"`
	}
	if err := Decode(strings.NewReader(`{"name":"ok"}`), &dst); err != nil || dst.Name != "ok" {
		t.Fatal(err)
	}
}
func TestDecodeLimit(t *testing.T) {
	var dst any
	if Decode(strings.NewReader(`{"x":"`+strings.Repeat("a", MaxBodyBytes)+`"}`), &dst) == nil {
		t.Fatal("accepted oversized body")
	}
}
func TestValidation(t *testing.T) {
	if ValidateString("email", "invalid") == nil {
		t.Fatal("invalid email accepted")
	}
	if _, _, err := Page("101", ""); err == nil {
		t.Fatal("invalid limit accepted")
	}
	if _, err := ID("0"); err == nil {
		t.Fatal("invalid ID accepted")
	}
	if len(RequestID()) != 32 {
		t.Fatal("request ID")
	}
}
