package audio

import "time"

const MaxUploadBytes int64 = 100 * 1024 * 1024

type Status string

const (
	StatusUploadPending Status = "UPLOAD_PENDING"
	StatusUploaded      Status = "UPLOADED"
	StatusScanning      Status = "SCANNING"
	StatusClean         Status = "CLEAN"
	StatusQuarantined   Status = "QUARANTINED"
	StatusScanFailed    Status = "SCAN_FAILED"
	StatusQueued        Status = "QUEUED"
	StatusTranscoding   Status = "TRANSCODING"
	StatusReady         Status = "READY"
	StatusFailed        Status = "TRANSCODE_FAILED"
	StatusDeleted       Status = "DELETED"
)

type Visibility string

const (
	VisibilityPrivate Visibility = "private"
	VisibilityPublic  Visibility = "public"
)

type Audio struct {
	ID                string     `json:"id"`
	OwnerSubject      string     `json:"-"`
	Title             string     `json:"title"`
	Visibility        Visibility `json:"visibility"`
	Status            Status     `json:"status"`
	UploadID          string     `json:"-"`
	SourceBucket      string     `json:"-"`
	SourceKey         string     `json:"-"`
	SourceVersion     string     `json:"-"`
	SourceChecksum    string     `json:"-"`
	SourceSize        int64      `json:"-"`
	SourceContentType string     `json:"-"`
	UploadVerified    bool       `json:"-"`
	ScanStatus        string     `json:"-"`
	DurationMS        *int64     `json:"duration_ms,omitempty"`
	PlaybackKey       string     `json:"-"`
	WaveformKey       string     `json:"-"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type CreateUploadInput struct {
	OwnerSubject   string
	Title          string
	ContentType    string
	ContentLength  int64
	ChecksumSHA256 string
}

type CreateUploadOutput struct {
	AudioID       string            `json:"audio_id"`
	UploadID      string            `json:"upload_id"`
	UploadURL     string            `json:"upload_url"`
	UploadHeaders map[string]string `json:"upload_headers"`
	ExpiresAt     time.Time         `json:"expires_at"`
}

type PlaybackAccess struct {
	AudioID     string    `json:"audio_id"`
	PlaybackURL string    `json:"playback_url"`
	WaveformURL string    `json:"waveform_url"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type SourceObject struct {
	VersionID      string
	ContentLength  int64
	ContentType    string
	ChecksumSHA256 string
}
