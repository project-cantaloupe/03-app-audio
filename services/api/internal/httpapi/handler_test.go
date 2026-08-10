package httpapi

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

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
