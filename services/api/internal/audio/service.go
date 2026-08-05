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
	MarkUploadVerified(context.Context, string, SourceObject, string, string, time.Time) (Audio, error)
}

type ObjectStore interface {
	PresignPut(context.Context, Audio, time.Duration) (string, error)
	HeadSource(context.Context, Audio) (SourceObject, error)
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
	artifactURLs   ArtifactURLSigner
	ids            IDGenerator
	clock          Clock
	bucket         string
	uploadExpiry   time.Duration
	playbackExpiry time.Duration
}

func NewService(repository Repository, objectStore ObjectStore, artifactURLs ArtifactURLSigner, ids IDGenerator, clock Clock, bucket string, uploadExpiry, playbackExpiry time.Duration) *Service {
	return &Service{
		repository: repository, objectStore: objectStore, artifactURLs: artifactURLs,
		ids: ids, clock: clock, bucket: bucket, uploadExpiry: uploadExpiry,
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
		UploadHeaders: map[string]string{
			"Content-Type":          input.ContentType,
			"x-amz-checksum-sha256": input.ChecksumSHA256,
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
	if record.Status != StatusUploadPending {
		if record.UploadVerified {
			return record, nil
		}
		return Audio{}, fmt.Errorf("%w: upload cannot be completed in status %s", ErrInvalidInput, record.Status)
	}

	object, err := s.objectStore.HeadSource(ctx, record)
	if err != nil {
		return Audio{}, fmt.Errorf("head uploaded object: %w", err)
	}
	if object.VersionID == "" ||
		(record.SourceVersion != "" && record.SourceVersion != object.VersionID) ||
		object.ContentLength != record.SourceSize ||
		normalizeContentType(object.ContentType) != record.SourceContentType ||
		object.ChecksumSHA256 != record.SourceChecksum {
		return Audio{}, ErrObjectMismatch
	}

	updated, err := s.repository.MarkUploadVerified(
		ctx,
		audioID,
		object,
		s.ids.New(),
		s.ids.New(),
		s.clock.Now().UTC(),
	)
	if err != nil {
		return Audio{}, fmt.Errorf("mark upload verified: %w", err)
	}
	return updated, nil
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
