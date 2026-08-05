package events

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type ScanResult struct {
	SchemaVersion int       `json:"schema_version"`
	EventID       string    `json:"event_id"`
	Bucket        string    `json:"bucket"`
	Key           string    `json:"key"`
	VersionID     string    `json:"version_id"`
	Status        string    `json:"status"`
	OccurredAt    time.Time `json:"occurred_at"`
}

func (r ScanResult) Validate() error {
	if r.SchemaVersion != 1 {
		return errors.New("unsupported scan-result schema_version")
	}
	if _, err := uuid.Parse(r.EventID); err != nil {
		return errors.New("scan-result event_id must be a UUID")
	}
	if strings.TrimSpace(r.Bucket) == "" || strings.TrimSpace(r.Key) == "" || strings.TrimSpace(r.VersionID) == "" {
		return errors.New("scan-result source fields are required")
	}
	switch r.Status {
	case "NO_THREATS_FOUND", "THREATS_FOUND", "UNSUPPORTED", "ACCESS_DENIED", "FAILED":
		return nil
	default:
		return fmt.Errorf("unsupported scan status %q", r.Status)
	}
}

type Artifacts struct {
	Bucket      string `json:"bucket"`
	PlaybackKey string `json:"playback_key"`
	WaveformKey string `json:"waveform_key"`
}

type ResultError struct {
	Code      string `json:"code"`
	Retryable bool   `json:"retryable"`
}

type TranscodeResult struct {
	SchemaVersion int          `json:"schema_version"`
	EventID       string       `json:"event_id"`
	JobID         string       `json:"job_id"`
	AudioID       string       `json:"audio_id"`
	Status        string       `json:"status"`
	Attempt       int          `json:"attempt"`
	OccurredAt    time.Time    `json:"occurred_at"`
	DurationMS    int64        `json:"duration_ms,omitempty"`
	Artifacts     *Artifacts   `json:"artifacts,omitempty"`
	Error         *ResultError `json:"error,omitempty"`
}

func (r TranscodeResult) Validate() error {
	if r.SchemaVersion != 1 {
		return errors.New("unsupported transcode-result schema_version")
	}
	for field, value := range map[string]string{"event_id": r.EventID, "job_id": r.JobID, "audio_id": r.AudioID} {
		if _, err := uuid.Parse(value); err != nil {
			return fmt.Errorf("transcode-result %s must be a UUID", field)
		}
	}
	if r.Attempt < 1 {
		return errors.New("transcode-result attempt must be positive")
	}
	switch r.Status {
	case "SUCCEEDED":
		if r.DurationMS <= 0 || r.Artifacts == nil || r.Artifacts.Bucket == "" || r.Artifacts.PlaybackKey == "" || r.Artifacts.WaveformKey == "" {
			return errors.New("successful transcode-result requires duration and artifacts")
		}
	case "FAILED":
		if r.Error == nil || strings.TrimSpace(r.Error.Code) == "" {
			return errors.New("failed transcode-result requires an error code")
		}
	default:
		return fmt.Errorf("unsupported transcode status %q", r.Status)
	}
	return nil
}

type OutboxEvent struct {
	ID            string
	EventType     string
	SchemaVersion int
	Payload       []byte
}
