package users

import (
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandler(t *testing.T) {
	h := NewHandler(NewService(NewMemoryStore()))
	router := fiber.New()
	h.Register(router)
	request := func(method, path, body string) (int, []byte) {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := router.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode, b
	}
	path := "/api/v1/users"
	body := `{ "name":"example","email":"person@example.com" }`
	status, response := request("POST", path, body)
	if status != 201 {
		t.Fatalf("create: %d %s", status, response)
	}
	var envelope struct {
		Data Record `json:"data"`
	}
	if err := json.Unmarshal(response, &envelope); err != nil || envelope.Data.ID < 1 {
		t.Fatalf("response: %s %v", response, err)
	}
	for _, invalid := range []string{`{}`, `null`, `{"unknown":true}`, body + body} {
		status, response = request("POST", path, invalid)
		if status != 400 {
			t.Fatalf("invalid body %s: %d %s", invalid, status, response)
		}
	}
	statusList, responseList := request("GET", path+"?limit=20", "")
	if statusList != 200 || !json.Valid(responseList) {
		t.Fatalf("list: %d %s", statusList, responseList)
	}
	if status, _ := request("GET", path+"?limit=101", ""); status != 400 {
		t.Fatal("pagination not validated")
	}

	h.service = NewService(failingStore{NewMemoryStore()})
	if status, b := request("POST", path, body); status != 500 || strings.Contains(string(b), failure.Error()) {
		t.Fatalf("storage error response: %d %s", status, b)
	}
}
