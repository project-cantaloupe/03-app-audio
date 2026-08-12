package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/project-cantaloupe/app-audio/services/api/internal/authn"
)

type stubAuthenticator struct {
	subject string
	err     error
}

func (authenticator stubAuthenticator) Subject(context.Context, string, string) (string, error) {
	return authenticator.subject, authenticator.err
}

func TestRequiredAuthenticationRejectsAnonymousRequest(t *testing.T) {
	handler := &Handler{
		auth:   stubAuthenticator{},
		logger: log.New(io.Discard, "", 0),
	}
	response := httptest.NewRecorder()
	handler.authenticate(func(http.ResponseWriter, *http.Request) {
		t.Fatal("anonymous request reached a protected handler")
	})(response, httptest.NewRequest(http.MethodPost, "/v1/audios/uploads", nil))

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
	if got := response.Header().Get("WWW-Authenticate"); got != `Bearer realm="audio-api"` {
		t.Fatalf("WWW-Authenticate = %q", got)
	}
}

func TestDisabledAuthenticationRejectsForgedCredentials(t *testing.T) {
	handler := &Handler{
		auth:   authn.Disabled{},
		logger: log.New(io.Discard, "", 0),
	}
	request := httptest.NewRequest(http.MethodPost, "/v1/audios/uploads", nil)
	request.Header.Set("Authorization", "Bearer forged")
	request.Header.Set("X-Cantaloupe-Subject", "forged-development-user")
	response := httptest.NewRecorder()

	handler.authenticate(func(http.ResponseWriter, *http.Request) {
		t.Fatal("forged credentials reached a protected handler")
	})(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestOptionalAuthenticationAllowsAnonymousRequest(t *testing.T) {
	handler := &Handler{
		auth:   stubAuthenticator{},
		logger: log.New(io.Discard, "", 0),
	}
	response := httptest.NewRecorder()
	handler.optionalAuthenticate(func(w http.ResponseWriter, request *http.Request) {
		if got := subject(request); got != "" {
			t.Fatalf("subject = %q, want anonymous", got)
		}
		w.WriteHeader(http.StatusNoContent)
	})(response, httptest.NewRequest(http.MethodGet, "/v1/audios?scope=public", nil))

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
}

func TestOptionalAuthenticationRejectsInvalidToken(t *testing.T) {
	handler := &Handler{
		auth:   stubAuthenticator{err: authn.ErrInvalidCredentials},
		logger: log.New(io.Discard, "", 0),
	}
	response := httptest.NewRecorder()
	handler.optionalAuthenticate(func(http.ResponseWriter, *http.Request) {
		t.Fatal("invalid token reached a public handler")
	})(response, httptest.NewRequest(http.MethodGet, "/v1/audios?scope=public", nil))

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestAuthenticationAddsSubjectToRequestContext(t *testing.T) {
	handler := &Handler{
		auth:   stubAuthenticator{subject: "user-123"},
		logger: log.New(io.Discard, "", 0),
	}
	response := httptest.NewRecorder()
	handler.authenticate(func(w http.ResponseWriter, request *http.Request) {
		if got := subject(request); got != "user-123" {
			t.Fatalf("subject = %q, want user-123", got)
		}
		w.WriteHeader(http.StatusNoContent)
	})(response, httptest.NewRequest(http.MethodPatch, "/v1/audios/audio-1", nil))

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
}

func TestAnonymousUploadLimiterResetsAfterOneMinute(t *testing.T) {
	limiter := &anonymousUploadLimiter{}
	now := time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)

	for request := 0; request < anonymousUploadsPerMinute; request++ {
		if !limiter.allow(now) {
			t.Fatalf("request %d was unexpectedly rate limited", request+1)
		}
	}
	if limiter.allow(now) {
		t.Fatal("request above the per-minute limit was allowed")
	}
	if !limiter.allow(now.Add(time.Minute)) {
		t.Fatal("limiter did not reset after one minute")
	}
}

func TestRequestLogEmitsStructuredFields(t *testing.T) {
	var output bytes.Buffer
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/audios/{audio_id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	handler := requestLog(log.New(&output, "", 0), mux)

	request := httptest.NewRequest(http.MethodGet, "/v1/audios/audio-1", nil)
	requestID := uuid.NewString()
	request.Header.Set("X-Request-ID", requestID)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if got := response.Header().Get("X-Request-ID"); got != requestID {
		t.Fatalf("X-Request-ID = %q, want %q", got, requestID)
	}
	var event map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &event); err != nil {
		t.Fatalf("decode structured log: %v", err)
	}
	for key, want := range map[string]any{
		"event_type":  "http_request_completed",
		"status":      "rejected",
		"request_id":  requestID,
		"http_method": http.MethodGet,
		"http_route":  "/v1/audios/{audio_id}",
		"http_status": float64(http.StatusNotFound),
	} {
		if got := event[key]; got != want {
			t.Errorf("%s = %#v, want %#v", key, got, want)
		}
	}
	if _, ok := event["processing_duration_ms"].(float64); !ok {
		t.Errorf("processing_duration_ms = %#v, want number", event["processing_duration_ms"])
	}
}
