package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/project-cantaloupe/app-audio/services/api/internal/audio"
	"github.com/project-cantaloupe/app-audio/services/api/internal/health"
	"github.com/project-cantaloupe/app-audio/services/api/internal/observability"
)

type Handler struct {
	service *audio.Service
	probe   *health.Probe
	logger  *log.Logger
}

func New(service *audio.Service, probe *health.Probe, logger *log.Logger) http.Handler {
	h := &Handler{service: service, probe: probe, logger: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.health)
	mux.HandleFunc("GET /readyz", h.ready)
	mux.HandleFunc("POST /v1/audios/uploads", h.createUpload)
	mux.HandleFunc("POST /v1/audios/{audio_id}/complete", h.completeUpload)
	mux.HandleFunc("GET /v1/audios", h.listAudios)
	mux.HandleFunc("GET /v1/audios/{audio_id}", h.getAudio)
	mux.HandleFunc("PATCH /v1/audios/{audio_id}", h.updateAudio)
	mux.HandleFunc("GET /v1/audios/{audio_id}/playback", h.getPlayback)
	return requestLog(logger, mux)
}

type createUploadRequest struct {
	Title          string `json:"title"`
	ContentType    string `json:"content_type"`
	ContentLength  int64  `json:"content_length"`
	ChecksumSHA256 string `json:"checksum_sha256"`
	Visibility     string `json:"visibility"`
}

type updateAudioRequest struct {
	Visibility string `json:"visibility"`
}

func (h *Handler) createUpload(w http.ResponseWriter, r *http.Request) {
	var body createUploadRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}
	result, err := h.service.CreateUpload(r.Context(), audio.CreateUploadInput{
		OwnerSubject: subject(r), Title: body.Title, ContentType: body.ContentType,
		ContentLength: body.ContentLength, ChecksumSHA256: body.ChecksumSHA256,
		Visibility: audio.Visibility(body.Visibility),
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	observability.Event(h.logger, "info", "upload_session_created", "upload session created", map[string]any{
		"status": "pending", "request_id": requestID(r.Context()),
		"audio_id": result.AudioID, "input_bytes": body.ContentLength,
		"content_type": body.ContentType, "visibility": body.Visibility,
	})
	writeJSON(w, http.StatusCreated, result)
}

func (h *Handler) completeUpload(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.CompleteUpload(r.Context(), subject(r), r.PathValue("audio_id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	observability.Event(h.logger, "info", "upload_completed", "uploaded source accepted", map[string]any{
		"request_id": requestID(r.Context()), "audio_id": result.ID,
		"status": result.Status, "input_bytes": result.SourceSize,
	})
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) listAudios(w http.ResponseWriter, r *http.Request) {
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "limit must be a positive integer")
			return
		}
		limit = parsed
	}

	// scope=public 은 공개 카탈로그다. 소유자를 가리지 않는 대신 저장소가
	// public + READY 로 좁힌다. 기본값은 요청자 본인의 트랙이다.
	scope := audio.ScopeOwner
	switch raw := r.URL.Query().Get("scope"); raw {
	case "", "owner":
	case "public":
		scope = audio.ScopePublic
	default:
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "scope must be owner or public")
		return
	}

	result, err := h.service.ListAudios(r.Context(), audio.ListAudiosInput{
		OwnerSubject: subject(r),
		Scope:        scope,
		Limit:        limit,
		Cursor:       r.URL.Query().Get("cursor"),
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) getAudio(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.GetAudio(r.Context(), subject(r), r.PathValue("audio_id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) updateAudio(w http.ResponseWriter, r *http.Request) {
	var body updateAudioRequest
	if err := decodeJSON(w, r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
		return
	}
	result, err := h.service.UpdateVisibility(r.Context(), audio.UpdateVisibilityInput{
		OwnerSubject: subject(r),
		AudioID:      r.PathValue("audio_id"),
		Visibility:   audio.Visibility(body.Visibility),
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) getPlayback(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.GetPlayback(r.Context(), subject(r), r.PathValue("audio_id"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) ready(w http.ResponseWriter, _ *http.Request) {
	if !h.probe.Ready() {
		writeError(w, http.StatusServiceUnavailable, "NOT_READY", "service is not ready")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func subject(r *http.Request) string {
	// 개발 단계 전용 경계다. Cognito 검증기가 추가되기 전에는 외부 배포에서
	// 이 헤더를 신뢰하면 안 된다.
	return strings.TrimSpace(r.Header.Get("X-Cantaloupe-Subject"))
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain exactly one JSON object")
	}
	return nil
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, audio.ErrUnauthorized):
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
	case errors.Is(err, audio.ErrForbidden):
		writeError(w, http.StatusForbidden, "FORBIDDEN", err.Error())
	case errors.Is(err, audio.ErrNotFound):
		writeError(w, http.StatusNotFound, "NOT_FOUND", err.Error())
	case errors.Is(err, audio.ErrNotReady):
		writeError(w, http.StatusConflict, "AUDIO_NOT_READY", err.Error())
	case errors.Is(err, audio.ErrInvalidInput), errors.Is(err, audio.ErrObjectMismatch):
		writeError(w, http.StatusUnprocessableEntity, "INVALID_REQUEST", err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "request could not be completed")
	}
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func requestLog(logger *log.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		id := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if _, err := uuid.Parse(id); err != nil {
			id = uuid.NewString()
		}
		w.Header().Set("X-Request-ID", id)
		r = r.WithContext(context.WithValue(r.Context(), requestIDContextKey{}, id))
		recorder := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)

		level, status := "info", "success"
		if recorder.status >= http.StatusInternalServerError {
			level, status = "error", "failed"
		} else if recorder.status >= http.StatusBadRequest {
			level, status = "warning", "rejected"
		}
		route := r.Pattern
		if _, pattern, found := strings.Cut(route, " "); found {
			route = pattern
		}
		if route == "" {
			route = r.URL.Path
		}
		observability.Event(logger, level, "http_request_completed", "http request completed", map[string]any{
			"status": status, "request_id": id,
			"http_method": r.Method, "http_route": route, "http_status": recorder.status,
			"processing_duration_ms": time.Since(started).Milliseconds(),
		})
	})
}

type requestIDContextKey struct{}

func requestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDContextKey{}).(string)
	return id
}

type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (w *responseRecorder) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
