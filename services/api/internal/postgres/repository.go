package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/project-cantaloupe/app-audio/services/api/internal/audio"
)

type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) CreateUpload(ctx context.Context, record audio.Audio) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO audios (
			id, owner_subject, title, visibility, status, upload_id,
			source_bucket, source_key, source_checksum, source_size,
			source_content_type, scan_status, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10,
			$11, $12, $13, $14
		)`,
		record.ID, record.OwnerSubject, record.Title, record.Visibility, record.Status,
		record.UploadID, record.SourceBucket, record.SourceKey, record.SourceChecksum,
		record.SourceSize, record.SourceContentType, record.ScanStatus,
		record.CreatedAt, record.UpdatedAt,
	)
	return err
}

func (r *Repository) GetAudio(ctx context.Context, id string) (audio.Audio, error) {
	row := r.pool.QueryRow(ctx, selectAudio+` WHERE id = $1 AND deleted_at IS NULL`, id)
	record, err := scanAudio(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return audio.Audio{}, audio.ErrNotFound
	}
	return record, err
}

// 정렬과 WHERE 절이 audios_owner_created_idx (owner_subject, created_at DESC)
// WHERE deleted_at IS NULL 과 일치한다.
//
// 행 비교 (created_at, id) < ($2, $3) 는 두 값을 사전순으로 함께 비교한다.
// created_at 만으로 자르면 같은 시각에 만들어진 행이 누락되거나 반복된다.
func (r *Repository) ListAudiosByOwner(
	ctx context.Context, ownerSubject string, limit int, after *audio.ListCursor,
) ([]audio.Audio, error) {
	var createdAt any
	var id any
	if after != nil {
		createdAt = after.CreatedAt
		id = after.ID
	}

	rows, err := r.pool.Query(ctx, selectAudio+`
		WHERE owner_subject = $1
			AND deleted_at IS NULL
			AND ($2::timestamptz IS NULL OR (created_at, id) < ($2, $3))
		ORDER BY created_at DESC, id DESC
		LIMIT $4`,
		ownerSubject, createdAt, id, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return collectAudios(rows, limit)
}

// 공개 카탈로그다. 소유자를 가리지 않는 대신 public과 READY로 좁힌다.
// 처리 중이거나 비공개인 트랙이 남에게 노출되지 않는 경계가 여기 한 곳이다.
//
// audios_public_created_idx 가 이 조건을 그대로 담고 있다 (002 마이그레이션).
func (r *Repository) ListPublicAudios(
	ctx context.Context, limit int, after *audio.ListCursor,
) ([]audio.Audio, error) {
	var createdAt any
	var id any
	if after != nil {
		createdAt = after.CreatedAt
		id = after.ID
	}

	rows, err := r.pool.Query(ctx, selectAudio+`
		WHERE visibility = 'public'
			AND status = 'READY'
			AND deleted_at IS NULL
			AND ($1::timestamptz IS NULL OR (created_at, id) < ($1, $2))
		ORDER BY created_at DESC, id DESC
		LIMIT $3`,
		createdAt, id, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return collectAudios(rows, limit)
}

func (r *Repository) UpdateVisibility(
	ctx context.Context, id string, visibility audio.Visibility, now time.Time,
) (audio.Audio, error) {
	row := r.pool.QueryRow(ctx, `
		UPDATE audios
		SET visibility = $2, updated_at = $3
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING `+audioColumns,
		id, string(visibility), now,
	)
	record, err := scanAudio(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return audio.Audio{}, audio.ErrNotFound
	}
	return record, err
}

func collectAudios(rows pgx.Rows, limit int) ([]audio.Audio, error) {
	records := make([]audio.Audio, 0, limit)
	for rows.Next() {
		record, err := scanAudio(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (r *Repository) MarkUploadVerified(ctx context.Context, id string, object audio.SourceObject, jobID, eventID string, now time.Time) (audio.Audio, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return audio.Audio{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	row := tx.QueryRow(ctx, `
		UPDATE audios
		SET upload_verified = TRUE,
			source_version = COALESCE(source_version, $2),
			status = CASE
				WHEN source_version IS NOT NULL AND source_version <> $2 THEN 'SCAN_FAILED'
				WHEN scan_status = 'NO_THREATS_FOUND' THEN 'CLEAN'
				WHEN scan_status = 'THREATS_FOUND' THEN 'QUARANTINED'
				WHEN scan_status IN ('UNSUPPORTED', 'ACCESS_DENIED', 'FAILED') THEN 'SCAN_FAILED'
				ELSE 'SCANNING'
			END,
			updated_at = $3
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING `+audioColumns,
		id, object.VersionID, now,
	)
	record, err := scanAudio(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return audio.Audio{}, audio.ErrNotFound
	}
	if err != nil {
		return audio.Audio{}, err
	}
	if record.Status == audio.StatusClean {
		if err := enqueueInitialTranscode(ctx, tx, record, jobID, eventID, now); err != nil {
			return audio.Audio{}, err
		}
		record.Status = audio.StatusQueued
	}
	if err := tx.Commit(ctx); err != nil {
		return audio.Audio{}, err
	}
	return record, nil
}

func enqueueInitialTranscode(ctx context.Context, tx pgx.Tx, record audio.Audio, jobID, eventID string, now time.Time) error {
	targetSpec := `{"mp3_bitrate_kbps":192,"waveform_points_per_second":20}`
	payload := fmt.Sprintf(
		`{"schema_version":1,"event_id":%q,"job_id":%q,"audio_id":%q,"source":{"bucket":%q,"key":%q,"version_id":%q,"checksum_sha256":%q},"targets":%s}`,
		eventID, jobID, record.ID, record.SourceBucket, record.SourceKey,
		record.SourceVersion, record.SourceChecksum, targetSpec,
	)
	commandTag, err := tx.Exec(ctx, `
		UPDATE audios SET status = 'QUEUED', updated_at = $2
		WHERE id = $1 AND status = 'CLEAN'`, record.ID, now)
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() == 0 {
		return nil
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO transcode_jobs (
			id, audio_id, request_key, schema_version, status, target_spec,
			created_at, updated_at
		) VALUES ($1, $2, 'initial', 1, 'QUEUED', $3::jsonb, $4, $4)
		ON CONFLICT (audio_id, request_key) DO NOTHING`,
		jobID, record.ID, targetSpec, now,
	); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO outbox_events (
			id, aggregate_id, event_type, schema_version, payload, created_at
		) VALUES ($1, $2, 'TranscodeRequested', 1, $3::jsonb, $4)
		ON CONFLICT (id) DO NOTHING`,
		eventID, record.ID, payload, now,
	)
	return err
}

const audioColumns = `
	id, owner_subject, title, visibility, status, upload_id,
	source_bucket, source_key, COALESCE(source_version, ''), source_checksum,
	source_size, source_content_type, upload_verified, scan_status,
	duration_ms, COALESCE(playback_key, ''), COALESCE(waveform_key, ''),
	created_at, updated_at`

const selectAudio = `SELECT ` + audioColumns + ` FROM audios`

type rowScanner interface {
	Scan(...any) error
}

func scanAudio(row rowScanner) (audio.Audio, error) {
	var record audio.Audio
	err := row.Scan(
		&record.ID, &record.OwnerSubject, &record.Title, &record.Visibility, &record.Status,
		&record.UploadID, &record.SourceBucket, &record.SourceKey, &record.SourceVersion,
		&record.SourceChecksum, &record.SourceSize, &record.SourceContentType,
		&record.UploadVerified, &record.ScanStatus, &record.DurationMS, &record.PlaybackKey,
		&record.WaveformKey, &record.CreatedAt, &record.UpdatedAt,
	)
	if err != nil {
		return audio.Audio{}, fmt.Errorf("scan audio: %w", err)
	}
	return record, nil
}
