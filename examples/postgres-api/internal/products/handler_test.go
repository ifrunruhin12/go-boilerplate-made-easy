package products

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandler(t *testing.T) {
	h := NewHandler(NewService(NewMemoryStore()))
	router := http.NewServeMux()
	h.Register(router)
	request := func(method, path, body string) (int, []byte) {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w.Code, w.Body.Bytes()
	}
	path := "/api/v1/products"
	body := `{ "name":"example","price":1.5,"active":true }`
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
	if status, _ := request("GET", path+"/99999", ""); status != 404 {
		t.Fatalf("missing record: %d", status)
	}
	if status, _ := request("GET", path+"/invalid", ""); status != 400 {
		t.Fatalf("invalid id: %d", status)
	}
	if status, _ := request("PUT", path+"/1", `{ "name":"updated","price":0,"active":false }`); status != 200 {
		t.Fatalf("update: %d", status)
	}
	if status, _ := request("DELETE", path+"/1", ""); status != 204 {
		t.Fatalf("delete: %d", status)
	}

	h.service = NewService(failingStore{NewMemoryStore()})
	if status, b := request("POST", path, body); status != 500 || strings.Contains(string(b), failure.Error()) {
		t.Fatalf("storage error response: %d %s", status, b)
	}
}
