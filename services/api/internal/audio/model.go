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
	// 비워두면 private로 만든다. 공개는 명시적 선택이어야 한다.
	Visibility Visibility
}

type CreateUploadOutput struct {
	AudioID       string            `json:"audio_id"`
	UploadID      string            `json:"upload_id"`
	UploadURL     string            `json:"upload_url"`
	UploadHeaders map[string]string `json:"upload_headers"`
	ExpiresAt     time.Time         `json:"expires_at"`
}

// 목록 조회의 기본값과 상한이다. audios_owner_created_idx가 소유자별 최신순을
// 받쳐주므로 정렬은 (created_at DESC, id DESC)로 고정한다.
const (
	DefaultListLimit = 20
	MaxListLimit     = 100
)

// ListCursor는 Keyset Pagination의 기준점이다. OFFSET을 쓰지 않는 이유는
// 목록을 넘기는 도중에 새 업로드가 들어오면 행이 밀려 중복·누락이 생기기
// 때문이다. 마지막으로 본 행을 기준으로 잡으면 그 문제가 없다.
type ListCursor struct {
	CreatedAt time.Time
	ID        string
}

// ListScope는 목록의 대상을 정한다.
//
// ScopeOwner   요청자 본인의 트랙. 상태와 무관하게 전부
// ScopePublic  공개 카탈로그. 소유자와 무관하되 public + READY만
type ListScope string

const (
	ScopeOwner  ListScope = "owner"
	ScopePublic ListScope = "public"
)

type ListAudiosInput struct {
	OwnerSubject string
	Scope        ListScope
	Limit        int
	Cursor       string
}

type UpdateVisibilityInput struct {
	OwnerSubject string
	AudioID      string
	Visibility   Visibility
}

// NextCursor가 비어 있으면 마지막 페이지다.
type AudioPage struct {
	Items      []Audio `json:"items"`
	NextCursor string  `json:"next_cursor,omitempty"`
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

type SourceScanRequest struct {
	EventID    string
	Bucket     string
	Key        string
	VersionID  string
	Status     string
	OccurredAt time.Time
}
