package server

import (
	"encoding/json"
	"io"
	"minimal/internal/config"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestServer(t *testing.T) {
	cfg := config.Config{CORSOrigins: []string{"https://allowed.example"}, ReadTimeout: time.Second, WriteTimeout: time.Second, IdleTimeout: time.Second}
	srv := New(cfg, Dependencies{})
	for _, tc := range []struct {
		method, path, origin string
		status               int
	}{{"GET", "/health", "", 200}, {"GET", "/missing", "", 404}, {"GET", "/health", "https://denied.example", 403}, {"OPTIONS", "/health", "https://allowed.example", 204}, {"GET", "/ready", "", 200}} {
		t.Run(tc.method+tc.path+tc.origin, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(""))
			req.Header.Set("Origin", tc.origin)
			w := httptest.NewRecorder()
			srv.HTTP.Handler.ServeHTTP(w, req)
			resp := w.Result()
			defer resp.Body.Close()
			b, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode != tc.status {
				t.Fatalf("%d %s", resp.StatusCode, b)
			}
			if len(resp.Header.Get("X-Request-ID")) != 32 {
				t.Fatal("missing request ID")
			}
			if tc.status != 204 && !json.Valid(b) {
				t.Fatalf("invalid JSON: %s", b)
			}
			if tc.origin == "https://allowed.example" && resp.Header.Get("Access-Control-Allow-Origin") != tc.origin {
				t.Fatal("missing CORS header")
			}
		})
	}
}
