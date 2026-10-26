package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Address                                                 string
	CORSOrigins                                             []string
	ReadTimeout, WriteTimeout, IdleTimeout, ShutdownTimeout time.Duration
}

func env(key, def string) string {
	if s := os.Getenv(key); s != "" {
		return s
	}
	return def
}
func Load() (Config, error) {
	port := env("PORT", "8080")
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return Config{}, fmt.Errorf("PORT must be an integer from 1 to 65535")
	}
	c := Config{Address: net.JoinHostPort(env("HOST", "0.0.0.0"), port)}
	for _, v := range []struct {
		key, def string
		target   *time.Duration
	}{{"READ_TIMEOUT", "10s", &c.ReadTimeout}, {"WRITE_TIMEOUT", "15s", &c.WriteTimeout}, {"IDLE_TIMEOUT", "60s", &c.IdleTimeout}, {"SHUTDOWN_TIMEOUT", "10s", &c.ShutdownTimeout}} {
		d, err := time.ParseDuration(env(v.key, v.def))
		if err != nil || d <= 0 {
			return c, fmt.Errorf("%s must be a positive Go duration", v.key)
		}
		*v.target = d
	}
	for _, origin := range strings.Split(os.Getenv("CORS_ORIGINS"), ",") {
		origin = strings.TrimSpace(origin)
		if origin == "" {
			continue
		}
		u, err := url.Parse(origin)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
			return c, fmt.Errorf("CORS_ORIGINS must contain exact http(s) origins without paths")
		}
		c.CORSOrigins = append(c.CORSOrigins, origin)
	}
	return c, nil
}
