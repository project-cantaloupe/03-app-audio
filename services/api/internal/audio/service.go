package audio

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Repository interface {
	CreateUpload(context.Context, Audio) error
	GetAudio(context.Context, string) (Audio, error)
	ListAudiosByOwner(context.Context, string, int, *ListCursor) ([]Audio, error)
	MarkUploadVerified(context.Context, string, SourceObject, string, string, time.Time) (Audio, error)
}

type ObjectStore interface {
	PresignPut(context.Context, Audio, time.Duration) (string, error)
	HeadSource(context.Context, Audio) (SourceObject, error)
}

type SourceScanAdapter interface {
	Submit(context.Context, SourceScanRequest) error
}

type ArtifactURLSigner interface {
	Sign(context.Context, string, time.Time) (string, error)
}

type IDGenerator interface {
	New() string
}

type Clock interface {
	Now() time.Time
}

type Service struct {
	repository     Repository
	objectStore    ObjectStore
	scanAdapter    SourceScanAdapter
	artifactURLs   ArtifactURLSigner
	ids            IDGenerator
	clock          Clock
	bucket         string
	uploadExpiry   time.Duration
	playbackExpiry time.Duration
}

func NewService(repository Repository, objectStore ObjectStore, scanAdapter SourceScanAdapter, artifactURLs ArtifactURLSigner, ids IDGenerator, clock Clock, bucket string, uploadExpiry, playbackExpiry time.Duration) *Service {
	return &Service{
		repository: repository, objectStore: objectStore, scanAdapter: scanAdapter,
		artifactURLs: artifactURLs,
		ids:          ids, clock: clock, bucket: bucket, uploadExpiry: uploadExpiry,
		playbackExpiry: playbackExpiry,
	}
}

func (s *Service) CreateUpload(ctx context.Context, input CreateUploadInput) (CreateUploadOutput, error) {
	input.OwnerSubject = strings.TrimSpace(input.OwnerSubject)
	input.Title = strings.TrimSpace(input.Title)
	input.ContentType = normalizeContentType(input.ContentType)

	if input.OwnerSubject == "" {
		return CreateUploadOutput{}, ErrUnauthorized
	}
	if input.Title == "" || len([]rune(input.Title)) > 200 {
		return CreateUploadOutput{}, fmt.Errorf("%w: title must contain 1 to 200 characters", ErrInvalidInput)
	}
	if input.ContentLength <= 0 || input.ContentLength > MaxUploadBytes {
		return CreateUploadOutput{}, fmt.Errorf("%w: content length must be between 1 and %d bytes", ErrInvalidInput, MaxUploadBytes)
	}
	if !allowedContentType(input.ContentType) {
		return CreateUploadOutput{}, fmt.Errorf("%w: unsupported content type", ErrInvalidInput)
	}
	if !validSHA256(input.ChecksumSHA256) {
		return CreateUploadOutput{}, fmt.Errorf("%w: checksum must be a base64-encoded SHA-256 digest", ErrInvalidInput)
	}

	now := s.clock.Now().UTC()
	audioID := s.ids.New()
	uploadID := s.ids.New()
	record := Audio{
		ID: audioID, OwnerSubject: input.OwnerSubject, Title: input.Title,
		Visibility: VisibilityPrivate, Status: StatusUploadPending, UploadID: uploadID,
		SourceBucket:   s.bucket,
		SourceKey:      fmt.Sprintf("incoming/%s/%s/source", audioID, uploadID),
		SourceChecksum: input.ChecksumSHA256, SourceSize: input.ContentLength,
		SourceContentType: input.ContentType, ScanStatus: "PENDING",
		CreatedAt: now, UpdatedAt: now,
	}

	if err := s.repository.CreateUpload(ctx, record); err != nil {
		return CreateUploadOutput{}, fmt.Errorf("create upload record: %w", err)
	}
	url, err := s.objectStore.PresignPut(ctx, record, s.uploadExpiry)
	if err != nil {
		return CreateUploadOutput{}, fmt.Errorf("presign upload: %w", err)
	}

	return CreateUploadOutput{
		AudioID:   audioID,
		UploadID:  uploadID,
		UploadURL: url,
		// Presign이 서명한 헤더만 돌려준다. 서명되지 않은 x-amz-* 헤더를
		// 하나라도 보내면 S3가 403으로 거부한다.
		UploadHeaders: map[string]string{
			"Content-Type": input.ContentType,
		},
		ExpiresAt: now.Add(s.uploadExpiry),
	}, nil
}

func (s *Service) CompleteUpload(ctx context.Context, ownerSubject, audioID string) (Audio, error) {
	if strings.TrimSpace(ownerSubject) == "" {
		return Audio{}, ErrUnauthorized
	}
	record, err := s.repository.GetAudio(ctx, audioID)
	if err != nil {
		return Audio{}, err
	}
	if record.OwnerSubject != ownerSubject {
		return Audio{}, ErrForbidden
	}
	if record.UploadVerified {
		return record, nil
	}
	if record.Status != StatusUploadPending && record.Status != StatusScanning {
		return Audio{}, fmt.Errorf("%w: upload cannot be completed in status %s", ErrInvalidInput, record.Status)
	}

	object, err := s.objectStore.HeadSource(ctx, record)
	if err != nil {
		return Audio{}, fmt.Errorf("head uploaded object: %w", err)
	}
	// SHA-256은 여기서 대조하지 않는다. Presign이 체크섬을 서명할 수 없어
	// S3가 이 값을 저장하지 않기 때문이다. 원본 무결성은 파일을 실제로
	// 내려받는 transcode 워커가 SOURCE_CHECKSUM_MISMATCH로 검증한다.
	if object.VersionID == "" ||
		(record.SourceVersion != "" && record.SourceVersion != object.VersionID) ||
		object.ContentLength != record.SourceSize ||
		normalizeContentType(object.ContentType) != record.SourceContentType {
		return Audio{}, ErrObjectMismatch
	}

	now := s.clock.Now().UTC()
	if err := s.scanAdapter.Submit(ctx, SourceScanRequest{
		EventID: record.UploadID, Bucket: record.SourceBucket, Key: record.SourceKey,
		VersionID: object.VersionID, Status: "NO_THREATS_FOUND", OccurredAt: now,
	}); err != nil {
		return Audio{}, fmt.Errorf("submit development scan result: %w", err)
	}

	updated, err := s.repository.MarkUploadVerified(
		ctx,
		audioID,
		object,
		s.ids.New(),
		s.ids.New(),
		now,
	)
	if err != nil {
		return Audio{}, fmt.Errorf("mark upload verified: %w", err)
	}
	return updated, nil
}

// ListAudios는 소유자 본인의 트랙만 최신순으로 돌려준다. 공개 피드가 아니다.
//
// 처리 중인 트랙도 함께 반환한다. 업로드 직후 SCANNING 상태가 목록에서 사라지면
// 사용자는 업로드가 실패했다고 판단하게 된다.
func (s *Service) ListAudios(ctx context.Context, input ListAudiosInput) (AudioPage, error) {
	if strings.TrimSpace(input.OwnerSubject) == "" {
		return AudioPage{}, ErrUnauthorized
	}

	limit := input.Limit
	switch {
	case limit <= 0:
		limit = DefaultListLimit
	case limit > MaxListLimit:
		limit = MaxListLimit
	}

	cursor, err := decodeListCursor(input.Cursor)
	if err != nil {
		return AudioPage{}, err
	}

	// 한 건 더 읽어 다음 페이지 존재 여부를 판단한다. 별도 COUNT 쿼리를 돌리면
	// 그 사이에 행이 추가돼 결과가 어긋날 수 있다.
	records, err := s.repository.ListAudiosByOwner(ctx, input.OwnerSubject, limit+1, cursor)
	if err != nil {
		return AudioPage{}, err
	}

	page := AudioPage{Items: records}
	if len(records) > limit {
		page.Items = records[:limit]
		last := page.Items[len(page.Items)-1]
		page.NextCursor = encodeListCursor(ListCursor{CreatedAt: last.CreatedAt, ID: last.ID})
	}
	// 빈 목록도 null이 아니라 []로 직렬화한다. 클라이언트가 분기하지 않게 한다.
	if page.Items == nil {
		page.Items = []Audio{}
	}
	return page, nil
}

// 커서는 클라이언트에게 불투명한 값이다. 정렬 기준이 바뀌어도 클라이언트를
// 고치지 않도록 내부 형식을 노출하지 않는다.
func encodeListCursor(cursor ListCursor) string {
	raw := cursor.CreatedAt.UTC().Format(time.RFC3339Nano) + "|" + cursor.ID
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeListCursor(value string) (*ListCursor, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("%w: cursor is not valid", ErrInvalidInput)
	}
	parts := strings.SplitN(string(decoded), "|", 2)
	if len(parts) != 2 || parts[1] == "" {
		return nil, fmt.Errorf("%w: cursor is not valid", ErrInvalidInput)
	}
	createdAt, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return nil, fmt.Errorf("%w: cursor is not valid", ErrInvalidInput)
	}
	return &ListCursor{CreatedAt: createdAt, ID: parts[1]}, nil
}

func (s *Service) GetAudio(ctx context.Context, subject, audioID string) (Audio, error) {
	record, err := s.repository.GetAudio(ctx, audioID)
	if err != nil {
		return Audio{}, err
	}
	if record.Visibility == VisibilityPrivate && record.OwnerSubject != subject {
		return Audio{}, ErrNotFound
	}
	return record, nil
}

func (s *Service) GetPlayback(ctx context.Context, subject, audioID string) (PlaybackAccess, error) {
	record, err := s.GetAudio(ctx, subject, audioID)
	if err != nil {
		return PlaybackAccess{}, err
	}
	if record.Status != StatusReady {
		return PlaybackAccess{}, ErrNotReady
	}
	if record.PlaybackKey == "" || record.WaveformKey == "" {
		return PlaybackAccess{}, errors.New("ready audio is missing artifact keys")
	}

	expiresAt := s.clock.Now().UTC().Add(s.playbackExpiry)
	playbackURL, err := s.artifactURLs.Sign(ctx, record.PlaybackKey, expiresAt)
	if err != nil {
		return PlaybackAccess{}, fmt.Errorf("sign playback artifact: %w", err)
	}
	waveformURL, err := s.artifactURLs.Sign(ctx, record.WaveformKey, expiresAt)
	if err != nil {
		return PlaybackAccess{}, fmt.Errorf("sign waveform artifact: %w", err)
	}
	return PlaybackAccess{
		AudioID: record.ID, PlaybackURL: playbackURL, WaveformURL: waveformURL,
		ExpiresAt: expiresAt,
	}, nil
}

func allowedContentType(value string) bool {
	switch value {
	case "audio/mpeg", "audio/wav", "audio/x-wav", "audio/flac", "audio/aac", "audio/ogg":
		return true
	default:
		return false
	}
}

func normalizeContentType(value string) string {
	return strings.ToLower(strings.TrimSpace(strings.Split(value, ";")[0]))
}

func validSHA256(value string) bool {
	decoded, err := base64.StdEncoding.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}
