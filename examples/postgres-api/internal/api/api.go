package api

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/mail"
	"strconv"
	"strings"
	"unicode/utf8"
)

const MaxBodyBytes = 1 << 20

var ErrNotFound = errors.New("resource not found")

type Envelope struct {
	Data  any    `json:"data,omitempty"`
	Error *Error `json:"error,omitempty"`
}
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func Success(data any) Envelope { return Envelope{Data: data} }
func Failure(code, message string) Envelope {
	return Envelope{Error: &Error{Code: code, Message: message}}
}
func Write(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("response write failed", "error", err)
	}
}
func ErrorResponse(err error) (int, Envelope) {
	if errors.Is(err, ErrNotFound) {
		return 404, Failure("not_found", "resource not found")
	}
	slog.Error("operation failed", "error", err)
	return 500, Failure("internal_error", "an internal error occurred")
}

// Decode accepts exactly one JSON object, rejects unknown fields, and limits input.
func Decode(reader io.Reader, dst any) error {
	b, err := io.ReadAll(io.LimitReader(reader, MaxBodyBytes+1))
	if err != nil {
		return fmt.Errorf("cannot read body")
	}
	if len(b) > MaxBodyBytes {
		return fmt.Errorf("body exceeds 1 MiB")
	}
	b = bytes.TrimSpace(b)
	if len(b) == 0 || b[0] != '{' {
		return fmt.Errorf("body must be a JSON object")
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("invalid JSON body: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return fmt.Errorf("body must contain a single JSON object")
	}
	return nil
}
func ValidateString(name, value string) error {
	if strings.TrimSpace(value) == "" || utf8.RuneCountInString(value) > 255 {
		return fmt.Errorf("%s is required and must contain at most 255 characters", name)
	}
	if name == "email" {
		address, err := mail.ParseAddress(value)
		if err != nil || address.Address != value {
			return fmt.Errorf("email must be a valid email address")
		}
	}
	return nil
}
func ID(s string) (int64, error) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("id must be a positive integer")
	}
	return n, nil
}
func Page(limit, offset string) (int, int, error) {
	l, o := 20, 0
	var err error
	if limit != "" {
		l, err = strconv.Atoi(limit)
		if err != nil || l < 1 || l > 100 {
			return 0, 0, fmt.Errorf("limit must be between 1 and 100")
		}
	}
	if offset != "" {
		o, err = strconv.Atoi(offset)
		if err != nil || o < 0 || o > 1000000 {
			return 0, 0, fmt.Errorf("offset must be between 0 and 1000000")
		}
	}
	return l, o, nil
}
func RequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("cannot generate request ID")
	}
	return hex.EncodeToString(b[:])
}
func Allows(origins []string, origin string) bool {
	for _, allowed := range origins {
		if allowed == origin {
			return true
		}
	}
	return false
}
