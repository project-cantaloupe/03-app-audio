package audio

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"
	"time"
)

type fakeRepository struct{ record Audio }

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
	service := NewService(repository, fakeObjectStore{}, fakeArtifactSigner{}, &sequenceIDs{values: []string{"audio-id", "upload-id"}}, clock, "cntlp-aws-quarantine", 15*time.Minute, 3*time.Hour)

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
	if result.UploadHeaders["x-amz-checksum-sha256"] != validChecksum() {
		t.Fatal("signed checksum header missing")
	}
	if !result.ExpiresAt.Equal(clock.value.Add(15 * time.Minute)) {
		t.Fatalf("unexpected expiry: %s", result.ExpiresAt)
	}
}

func TestCreateUploadRejectsOversizedObject(t *testing.T) {
	service := NewService(&fakeRepository{}, fakeObjectStore{}, fakeArtifactSigner{}, &sequenceIDs{}, fixedClock{}, "cntlp-aws-quarantine", 15*time.Minute, 3*time.Hour)
	_, err := service.CreateUpload(context.Background(), CreateUploadInput{
		OwnerSubject: "owner", Title: "title", ContentType: "audio/wav",
		ContentLength: MaxUploadBytes + 1, ChecksumSHA256: validChecksum(),
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestCompleteUploadRejectsChecksumMismatch(t *testing.T) {
	checksum := validChecksum()
	repository := &fakeRepository{record: Audio{
		ID: "audio-id", OwnerSubject: "owner", Status: StatusUploadPending,
		SourceSize: 10, SourceContentType: "audio/mpeg", SourceChecksum: checksum,
	}}
	service := NewService(repository, fakeObjectStore{object: SourceObject{
		VersionID: "v1", ContentLength: 10, ContentType: "audio/mpeg", ChecksumSHA256: "different",
	}}, fakeArtifactSigner{}, &sequenceIDs{values: []string{"job-id", "event-id"}}, fixedClock{}, "cntlp-aws-quarantine", 15*time.Minute, 3*time.Hour)

	_, err := service.CompleteUpload(context.Background(), "owner", "audio-id")
	if !errors.Is(err, ErrObjectMismatch) {
		t.Fatalf("expected ErrObjectMismatch, got %v", err)
	}
}

func TestGetPlaybackSignsReadyArtifacts(t *testing.T) {
	now := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	repository := &fakeRepository{record: Audio{
		ID: "audio-id", OwnerSubject: "owner", Visibility: VisibilityPrivate,
		Status: StatusReady, PlaybackKey: "audios/audio-id/playback.mp3",
		WaveformKey: "audios/audio-id/waveform.json",
	}}
	service := NewService(repository, fakeObjectStore{}, fakeArtifactSigner{}, &sequenceIDs{}, fixedClock{value: now}, "cntlp-aws-quarantine", 15*time.Minute, 3*time.Hour)

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
	service := NewService(repository, fakeObjectStore{}, fakeArtifactSigner{}, &sequenceIDs{}, fixedClock{}, "cntlp-aws-quarantine", 15*time.Minute, 3*time.Hour)

	_, err := service.GetPlayback(context.Background(), "owner", "audio-id")
	if !errors.Is(err, ErrNotReady) {
		t.Fatalf("expected ErrNotReady, got %v", err)
	}
}
