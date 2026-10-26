package server

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"postgres-api/internal/api"
	"postgres-api/internal/config"
	"time"

	"errors"
	"net/http"
)

type Dependencies struct {
	Logger *slog.Logger
	DB     *sql.DB
}

func ready(ctx context.Context, d Dependencies) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if d.DB == nil {
		return fmt.Errorf("database unavailable")
	}
	if err := d.DB.PingContext(ctx); err != nil {
		return err
	}
	return nil
}

type Server struct{ HTTP *http.Server }

func New(c config.Config, d Dependencies) *Server {
	if d.Logger == nil {
		d.Logger = slog.Default()
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		api.Write(w, 200, api.Success(map[string]string{"status": "ok"}))
	})
	mux.HandleFunc("GET /ready", func(w http.ResponseWriter, r *http.Request) {
		if err := ready(r.Context(), d); err != nil {
			d.Logger.Warn("readiness failed")
			api.Write(w, 503, api.Failure("not_ready", "dependencies unavailable"))
			return
		}
		api.Write(w, 200, api.Success(map[string]string{"status": "ready"}))
	})
	for _, path := range []string{"/health", "/ready"} {
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Allow", "GET")
			api.Write(w, 405, api.Failure("method_not_allowed", "method not allowed"))
		})
	}
	registerResources(mux, d)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		api.Write(w, 404, api.Failure("not_found", "route not found"))
	})
	return &Server{HTTP: &http.Server{Handler: middleware(mux, c, d.Logger), ReadTimeout: c.ReadTimeout, ReadHeaderTimeout: 5 * time.Second, WriteTimeout: c.WriteTimeout, IdleTimeout: c.IdleTimeout, MaxHeaderBytes: 1 << 20}}
}
func (s *Server) Listen(address string) error {
	s.HTTP.Addr = address
	err := s.HTTP.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
func (s *Server) Shutdown(ctx context.Context) error {
	if err := s.HTTP.Shutdown(ctx); err != nil {
		_ = s.HTTP.Close()
		return err
	}
	return nil
}

type recorder struct {
	http.ResponseWriter
	status int
}

func (w *recorder) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
		w.ResponseWriter.WriteHeader(status)
	}
}
func (w *recorder) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(200)
	}
	return w.ResponseWriter.Write(b)
}
func (w *recorder) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func middleware(next http.Handler, c config.Config, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &recorder{ResponseWriter: w}
		id := api.RequestID()
		rw.Header().Set("X-Request-ID", id)
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("handler panic", "request_id", id)
				if rw.status == 0 {
					api.Write(rw, 500, api.Failure("internal_error", "an internal error occurred"))
				}
			}
			status := rw.status
			if status == 0 {
				status = 200
			}
			logger.Info("request", "request_id", id, "method", r.Method, "path", r.URL.Path, "status", status, "duration", time.Since(start))
		}()
		rw.Header().Set("Vary", "Origin")
		if origin := r.Header.Get("Origin"); origin != "" {
			if !api.Allows(c.CORSOrigins, origin) {
				api.Write(rw, 403, api.Failure("cors_denied", "origin is not allowed"))
				return
			}
			rw.Header().Set("Access-Control-Allow-Origin", origin)
			rw.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
			rw.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization,X-Request-ID")
			rw.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
		}
		if r.Method == http.MethodOptions {
			rw.WriteHeader(204)
			return
		}
		r.Body = http.MaxBytesReader(rw, r.Body, api.MaxBodyBytes)
		ctx, cancel := context.WithTimeout(r.Context(), c.WriteTimeout)
		defer cancel()
		next.ServeHTTP(rw, r.WithContext(ctx))
	})
}
