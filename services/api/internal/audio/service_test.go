package audio

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"
)

type fakeRepository struct {
	record Audio

	// 목록 조회용. listResult를 그대로 돌려주고 받은 인자를 기록한다.
	listResult []Audio
	listOwner  string
	listLimit  int
	listCursor *ListCursor
}

func (f *fakeRepository) CreateUpload(_ context.Context, record Audio) error {
	f.record = record
	return nil
}
func (f *fakeRepository) GetAudio(_ context.Context, id string) (Audio, error) {
	if f.record.ID != id {
		return Audio{}, ErrNotFound
	}
	return f.record, nil
}
func (f *fakeRepository) ListAudiosByOwner(_ context.Context, owner string, limit int, after *ListCursor) ([]Audio, error) {
	f.listOwner, f.listLimit, f.listCursor = owner, limit, after
	if len(f.listResult) > limit {
		return f.listResult[:limit], nil
	}
	return f.listResult, nil
}
func (f *fakeRepository) MarkUploadVerified(_ context.Context, _ string, object SourceObject, _, _ string, now time.Time) (Audio, error) {
	f.record.UploadVerified = true
	f.record.SourceVersion = object.VersionID
	f.record.Status = StatusUploaded
	f.record.UpdatedAt = now
	return f.record, nil
}

type fakeObjectStore struct{ object SourceObject }

func (f fakeObjectStore) PresignPut(context.Context, Audio, time.Duration) (string, error) {
	return "https://upload.example.test", nil
}
func (f fakeObjectStore) HeadSource(context.Context, Audio) (SourceObject, error) {
	return f.object, nil
}

type fakeScanAdapter struct {
	request SourceScanRequest
	err     error
	calls   int
}

func (f *fakeScanAdapter) Submit(_ context.Context, request SourceScanRequest) error {
	f.calls++
	f.request = request
	return f.err
}

type fakeArtifactSigner struct{}

func (fakeArtifactSigner) Sign(_ context.Context, key string, _ time.Time) (string, error) {
	return "https://media.example.test/" + key, nil
}

type sequenceIDs struct {
	values []string
	index  int
}

func (s *sequenceIDs) New() string { value := s.values[s.index]; s.index++; return value }

type fixedClock struct{ value time.Time }

func (f fixedClock) Now() time.Time { return f.value }

func validChecksum() string { return base64.StdEncoding.EncodeToString(make([]byte, 32)) }

func TestCreateUploadBuildsImmutableSourceKey(t *testing.T) {
	repository := &fakeRepository{}
	clock := fixedClock{value: time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)}
	service := NewService(repository, fakeObjectStore{}, &fakeScanAdapter{}, fakeArtifactSigner{}, &sequenceIDs{values: []string{"audio-id", "upload-id"}}, clock, "cntlp-aws-quarantine", 15*time.Minute, 3*time.Hour)

	result, err := service.CreateUpload(context.Background(), CreateUploadInput{
		OwnerSubject: "cognito-sub", Title: " test audio ", ContentType: "audio/mpeg",
		ContentLength: 1024, ChecksumSHA256: validChecksum(),
	})
	if err != nil {
		t.Fatalf("CreateUpload() error = %v", err)
	}
	if repository.record.SourceKey != "incoming/audio-id/upload-id/source" {
		t.Fatalf("unexpected source key: %s", repository.record.SourceKey)
	}
	if result.UploadHeaders["Content-Type"] != "audio/mpeg" {
		t.Fatalf("unexpected content type header: %q", result.UploadHeaders["Content-Type"])
	}
	// Presign이 서명하지 못하는 헤더를 돌려주면 브라우저 업로드가 S3에서
	// 403으로 거부된다. 서명된 헤더만 나가야 한다.
	if len(result.UploadHeaders) != 1 {
		t.Fatalf("upload headers must contain only signed headers, got %v", result.UploadHeaders)
	}
	if !result.ExpiresAt.Equal(clock.value.Add(15 * time.Minute)) {
		t.Fatalf("unexpected expiry: %s", result.ExpiresAt)
	}
}

func TestCreateUploadRejectsOversizedObject(t *testing.T) {
	service := NewService(&fakeRepository{}, fakeObjectStore{}, &fakeScanAdapter{}, fakeArtifactSigner{}, &sequenceIDs{}, fixedClock{}, "cntlp-aws-quarantine", 15*time.Minute, 3*time.Hour)
	_, err := service.CreateUpload(context.Background(), CreateUploadInput{
		OwnerSubject: "owner", Title: "title", ContentType: "audio/wav",
		ContentLength: MaxUploadBytes + 1, ChecksumSHA256: validChecksum(),
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestCompleteUploadRejectsSizeMismatch(t *testing.T) {
	repository := &fakeRepository{record: Audio{
		ID: "audio-id", UploadID: "upload-id", OwnerSubject: "owner", Status: StatusUploadPending,
		SourceSize: 10, SourceContentType: "audio/mpeg", SourceChecksum: validChecksum(),
	}}
	service := NewService(repository, fakeObjectStore{object: SourceObject{
		VersionID: "v1", ContentLength: 11, ContentType: "audio/mpeg",
	}}, &fakeScanAdapter{}, fakeArtifactSigner{}, &sequenceIDs{values: []string{"job-id", "event-id"}}, fixedClock{}, "cntlp-aws-quarantine", 15*time.Minute, 3*time.Hour)

	_, err := service.CompleteUpload(context.Background(), "owner", "audio-id")
	if !errors.Is(err, ErrObjectMismatch) {
		t.Fatalf("expected ErrObjectMismatch, got %v", err)
	}
}

// S3는 Presigned PUT으로 올라온 객체에 SHA-256을 기록하지 않는다. 그 값이
// 비어 있다는 이유로 업로드를 거부하면 정상 업로드가 전부 막힌다.
// 무결성 대조는 원본을 내려받는 transcode 워커가 담당한다.
func TestCompleteUploadAcceptsObjectWithoutStoredChecksum(t *testing.T) {
	repository := &fakeRepository{record: Audio{
		ID: "audio-id", UploadID: "upload-id", OwnerSubject: "owner", Status: StatusUploadPending,
		SourceSize: 10, SourceContentType: "audio/mpeg", SourceChecksum: validChecksum(),
	}}
	service := NewService(repository, fakeObjectStore{object: SourceObject{
		VersionID: "v1", ContentLength: 10, ContentType: "audio/mpeg", ChecksumSHA256: "",
	}}, &fakeScanAdapter{}, fakeArtifactSigner{}, &sequenceIDs{values: []string{"job-id", "event-id"}}, fixedClock{}, "cntlp-aws-quarantine", 15*time.Minute, 3*time.Hour)

	if _, err := service.CompleteUpload(context.Background(), "owner", "audio-id"); err != nil {
		t.Fatalf("CompleteUpload() error = %v", err)
	}
}

func TestCompleteUploadSubmitsStableCleanResultBeforeVerification(t *testing.T) {
	now := time.Date(2026, 8, 5, 13, 0, 0, 0, time.UTC)
	checksum := validChecksum()
	repository := &fakeRepository{record: Audio{
		ID: "audio-id", UploadID: "upload-id", OwnerSubject: "owner",
		Status: StatusUploadPending, SourceBucket: "cntlp-aws-quarantine",
		SourceKey: "incoming/audio-id/upload-id/source", SourceSize: 10,
		SourceContentType: "audio/mpeg", SourceChecksum: checksum,
	}}
	scanAdapter := &fakeScanAdapter{}
	service := NewService(repository, fakeObjectStore{object: SourceObject{
		VersionID: "v1", ContentLength: 10, ContentType: "audio/mpeg",
		ChecksumSHA256: checksum,
	}}, scanAdapter, fakeArtifactSigner{}, &sequenceIDs{values: []string{"job-id", "event-id"}}, fixedClock{value: now}, "cntlp-aws-quarantine", 15*time.Minute, 3*time.Hour)

	result, err := service.CompleteUpload(context.Background(), "owner", "audio-id")
	if err != nil {
		t.Fatalf("CompleteUpload() error = %v", err)
	}
	if !result.UploadVerified {
		t.Fatal("upload was not marked verified")
	}
	if scanAdapter.request.EventID != "upload-id" {
		t.Fatalf("scan event must reuse stable upload ID, got %q", scanAdapter.request.EventID)
	}
	if scanAdapter.request.VersionID != "v1" || scanAdapter.request.Status != "NO_THREATS_FOUND" {
		t.Fatalf("unexpected scan request: %#v", scanAdapter.request)
	}
	if !scanAdapter.request.OccurredAt.Equal(now) {
		t.Fatalf("unexpected scan occurrence time: %s", scanAdapter.request.OccurredAt)
	}
}

func TestCompleteUploadDoesNotVerifyWhenScanSubmissionFails(t *testing.T) {
	checksum := validChecksum()
	repository := &fakeRepository{record: Audio{
		ID: "audio-id", UploadID: "upload-id", OwnerSubject: "owner",
		Status: StatusUploadPending, SourceBucket: "cntlp-aws-quarantine",
		SourceKey: "incoming/audio-id/upload-id/source", SourceSize: 10,
		SourceContentType: "audio/mpeg", SourceChecksum: checksum,
	}}
	service := NewService(repository, fakeObjectStore{object: SourceObject{
		VersionID: "v1", ContentLength: 10, ContentType: "audio/mpeg",
		ChecksumSHA256: checksum,
	}}, &fakeScanAdapter{err: errors.New("queue unavailable")}, fakeArtifactSigner{}, &sequenceIDs{}, fixedClock{}, "cntlp-aws-quarantine", 15*time.Minute, 3*time.Hour)

	_, err := service.CompleteUpload(context.Background(), "owner", "audio-id")
	if err == nil {
		t.Fatal("expected scan submission failure")
	}
	if repository.record.UploadVerified {
		t.Fatal("upload must remain unverified when scan submission fails")
	}
}

func TestCompleteUploadDoesNotRepublishVerifiedUpload(t *testing.T) {
	repository := &fakeRepository{record: Audio{
		ID: "audio-id", UploadID: "upload-id", OwnerSubject: "owner",
		Status: StatusScanning, UploadVerified: true,
	}}
	scanAdapter := &fakeScanAdapter{}
	service := NewService(repository, fakeObjectStore{}, scanAdapter, fakeArtifactSigner{}, &sequenceIDs{}, fixedClock{}, "cntlp-aws-quarantine", 15*time.Minute, 3*time.Hour)

	_, err := service.CompleteUpload(context.Background(), "owner", "audio-id")
	if err != nil {
		t.Fatalf("CompleteUpload() error = %v", err)
	}
	if scanAdapter.calls != 0 {
		t.Fatalf("verified upload republished scan result %d times", scanAdapter.calls)
	}
}

func TestGetPlaybackSignsReadyArtifacts(t *testing.T) {
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	repository := &fakeRepository{record: Audio{
		ID: "audio-id", OwnerSubject: "owner", Visibility: VisibilityPrivate,
		Status: StatusReady, PlaybackKey: "audios/audio-id/playback.mp3",
		WaveformKey: "audios/audio-id/waveform.json",
	}}
	service := NewService(repository, fakeObjectStore{}, &fakeScanAdapter{}, fakeArtifactSigner{}, &sequenceIDs{}, fixedClock{value: now}, "cntlp-aws-quarantine", 15*time.Minute, 3*time.Hour)

	result, err := service.GetPlayback(context.Background(), "owner", "audio-id")
	if err != nil {
		t.Fatalf("GetPlayback() error = %v", err)
	}
	if result.PlaybackURL != "https://media.example.test/audios/audio-id/playback.mp3" {
		t.Fatalf("unexpected playback URL: %s", result.PlaybackURL)
	}
	if !result.ExpiresAt.Equal(now.Add(3 * time.Hour)) {
		t.Fatalf("unexpected expiry: %s", result.ExpiresAt)
	}
}

func TestGetPlaybackRejectsAudioBeforeReady(t *testing.T) {
	repository := &fakeRepository{record: Audio{
		ID: "audio-id", OwnerSubject: "owner", Visibility: VisibilityPrivate,
		Status: StatusTranscoding,
	}}
	service := NewService(repository, fakeObjectStore{}, &fakeScanAdapter{}, fakeArtifactSigner{}, &sequenceIDs{}, fixedClock{}, "cntlp-aws-quarantine", 15*time.Minute, 3*time.Hour)

	_, err := service.GetPlayback(context.Background(), "owner", "audio-id")
	if !errors.Is(err, ErrNotReady) {
		t.Fatalf("expected ErrNotReady, got %v", err)
	}
}

func listService(repository *fakeRepository) *Service {
	return NewService(repository, fakeObjectStore{}, &fakeScanAdapter{}, fakeArtifactSigner{},
		&sequenceIDs{}, fixedClock{}, "cntlp-aws-quarantine", 15*time.Minute, 3*time.Hour)
}

func TestListAudiosRequiresSubject(t *testing.T) {
	_, err := listService(&fakeRepository{}).ListAudios(context.Background(), ListAudiosInput{})
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

// 다음 페이지 존재 여부를 알려면 요청한 개수보다 한 건 더 읽어야 한다.
func TestListAudiosReadsOneExtraRowToDetectNextPage(t *testing.T) {
	repository := &fakeRepository{}
	if _, err := listService(repository).ListAudios(context.Background(), ListAudiosInput{
		OwnerSubject: "owner", Limit: 5,
	}); err != nil {
		t.Fatalf("ListAudios() error = %v", err)
	}
	if repository.listLimit != 6 {
		t.Fatalf("expected limit 6, got %d", repository.listLimit)
	}
	if repository.listOwner != "owner" {
		t.Fatalf("expected owner scope, got %q", repository.listOwner)
	}
}

func TestListAudiosClampsLimit(t *testing.T) {
	for _, tc := range []struct{ in, want int }{
		{in: 0, want: DefaultListLimit + 1},
		{in: -3, want: DefaultListLimit + 1},
		{in: MaxListLimit + 50, want: MaxListLimit + 1},
	} {
		repository := &fakeRepository{}
		if _, err := listService(repository).ListAudios(context.Background(), ListAudiosInput{
			OwnerSubject: "owner", Limit: tc.in,
		}); err != nil {
			t.Fatalf("ListAudios() error = %v", err)
		}
		if repository.listLimit != tc.want {
			t.Fatalf("limit %d: expected %d, got %d", tc.in, tc.want, repository.listLimit)
		}
	}
}

// 마지막 페이지에서 커서를 주면 클라이언트가 빈 페이지를 한 번 더 요청한다.
func TestListAudiosOmitsCursorOnLastPage(t *testing.T) {
	repository := &fakeRepository{listResult: []Audio{{ID: "a"}, {ID: "b"}}}
	page, err := listService(repository).ListAudios(context.Background(), ListAudiosInput{
		OwnerSubject: "owner", Limit: 5,
	})
	if err != nil {
		t.Fatalf("ListAudios() error = %v", err)
	}
	if page.NextCursor != "" {
		t.Fatalf("expected no cursor, got %q", page.NextCursor)
	}
	if len(page.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(page.Items))
	}
}

func TestListAudiosReturnsCursorWhenMoreRemain(t *testing.T) {
	now := time.Date(2026, 8, 6, 9, 0, 0, 0, time.UTC)
	repository := &fakeRepository{listResult: []Audio{
		{ID: "a", CreatedAt: now}, {ID: "b", CreatedAt: now.Add(-time.Minute)},
		{ID: "c", CreatedAt: now.Add(-2 * time.Minute)},
	}}
	service := listService(repository)

	page, err := service.ListAudios(context.Background(), ListAudiosInput{OwnerSubject: "owner", Limit: 2})
	if err != nil {
		t.Fatalf("ListAudios() error = %v", err)
	}
	if len(page.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(page.Items))
	}
	if page.NextCursor == "" {
		t.Fatal("expected a cursor when more rows remain")
	}

	// 그 커서로 다시 요청하면 마지막으로 본 행이 기준점이 되어야 한다.
	if _, err := service.ListAudios(context.Background(), ListAudiosInput{
		OwnerSubject: "owner", Limit: 2, Cursor: page.NextCursor,
	}); err != nil {
		t.Fatalf("ListAudios() error = %v", err)
	}
	if repository.listCursor == nil {
		t.Fatal("expected the cursor to reach the repository")
	}
	if repository.listCursor.ID != "b" {
		t.Fatalf("expected cursor at last returned row, got %q", repository.listCursor.ID)
	}
	if !repository.listCursor.CreatedAt.Equal(now.Add(-time.Minute)) {
		t.Fatalf("unexpected cursor timestamp: %s", repository.listCursor.CreatedAt)
	}
}

func TestListAudiosRejectsMalformedCursor(t *testing.T) {
	for _, cursor := range []string{"not-base64!!", "YWJj", "fGlk"} {
		_, err := listService(&fakeRepository{}).ListAudios(context.Background(), ListAudiosInput{
			OwnerSubject: "owner", Cursor: cursor,
		})
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("cursor %q: expected ErrInvalidInput, got %v", cursor, err)
		}
	}
}

// 빈 결과가 null로 직렬화되면 클라이언트가 분기해야 한다.
func TestListAudiosReturnsEmptySliceNotNil(t *testing.T) {
	page, err := listService(&fakeRepository{}).ListAudios(context.Background(), ListAudiosInput{
		OwnerSubject: "owner",
	})
	if err != nil {
		t.Fatalf("ListAudios() error = %v", err)
	}
	if page.Items == nil {
		t.Fatal("expected an empty slice, got nil")
	}
}
