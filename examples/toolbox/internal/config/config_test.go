package config

import "testing"

func validEnv(t *testing.T) {
	t.Helper()
	for key, value := range map[string]string{"PORT": "8080", "HOST": "127.0.0.1", "CORS_ORIGINS": "", "READ_TIMEOUT": "10s", "WRITE_TIMEOUT": "15s", "IDLE_TIMEOUT": "60s", "SHUTDOWN_TIMEOUT": "10s", "DATABASE_URL": "postgres://app:local@localhost:5432/app?sslmode=disable", "DB_MAX_OPEN": "10", "DB_MAX_IDLE": "5", "REDIS_URL": "redis://localhost:6379/0"} {
		t.Setenv(key, value)
	}
}
func TestLoad(t *testing.T) {
	validEnv(t)
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Address != "127.0.0.1:8080" {
		t.Fatal(c.Address)
	}
}
func TestInvalidConfig(t *testing.T) {
	for _, tc := range []struct{ key, value string }{{"PORT", "-1"}, {"PORT", "abc"}, {"READ_TIMEOUT", "0s"}, {"CORS_ORIGINS", "*"}, {"CORS_ORIGINS", "https://example.com/path"}, {"DATABASE_URL", "mysql://localhost"}, {"DB_MAX_OPEN", "0"}, {"DB_MAX_IDLE", "11"}, {"REDIS_URL", "http://localhost"}} {
		t.Run(tc.key+tc.value, func(t *testing.T) {
			validEnv(t)
			t.Setenv(tc.key, tc.value)
			if _, err := Load(); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
