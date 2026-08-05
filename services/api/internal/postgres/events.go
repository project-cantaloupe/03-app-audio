package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/project-cantaloupe/app-audio/services/api/internal/audio"
	"github.com/project-cantaloupe/app-audio/services/api/internal/events"
)

func (r *Repository) ProcessScanResult(ctx context.Context, result events.ScanResult, jobID, outboxID string, now time.Time) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	inserted, err := registerProcessedEvent(ctx, tx, result.EventID, "ScanResult", now)
	if err != nil || !inserted {
		if err != nil {
			return err
		}
		return tx.Commit(ctx)
	}

	record, err := scanAudio(tx.QueryRow(ctx,
		selectAudio+` WHERE source_bucket = $1 AND source_key = $2 AND deleted_at IS NULL FOR UPDATE`,
		result.Bucket, result.Key,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return audio.ErrNotFound
	}
	if err != nil {
		return err
	}

	status := audio.StatusScanning
	if record.SourceVersion != "" && record.SourceVersion != result.VersionID {
		result.Status = "FAILED"
	}
	switch result.Status {
	case "NO_THREATS_FOUND":
		if record.UploadVerified {
			status = audio.StatusClean
		}
	case "THREATS_FOUND":
		status = audio.StatusQuarantined
	default:
		status = audio.StatusScanFailed
	}

	row := tx.QueryRow(ctx, `
		UPDATE audios
		SET source_version = COALESCE(source_version, $2),
			scan_status = $3,
			status = $4,
			updated_at = $5
		WHERE id = $1
		RETURNING `+audioColumns,
		record.ID, result.VersionID, result.Status, status, now,
	)
	record, err = scanAudio(row)
	if err != nil {
		return err
	}
	if record.Status == audio.StatusClean {
		if err := enqueueInitialTranscode(ctx, tx, record, jobID, outboxID, now); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *Repository) ProcessTranscodeResult(ctx context.Context, result events.TranscodeResult, maximumAttempts int, now time.Time) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	inserted, err := registerProcessedEvent(ctx, tx, result.EventID, "TranscodeResult", now)
	if err != nil || !inserted {
		if err != nil {
			return err
		}
		return tx.Commit(ctx)
	}

	var audioID string
	if err := tx.QueryRow(ctx, `
		SELECT audio_id FROM transcode_jobs
		WHERE id = $1 FOR UPDATE`, result.JobID,
	).Scan(&audioID); errors.Is(err, pgx.ErrNoRows) {
		return audio.ErrNotFound
	} else if err != nil {
		return err
	}
	if audioID != result.AudioID {
		return errors.New("transcode result audio_id does not match the job")
	}

	if result.Status == "SUCCEEDED" {
		if _, err := tx.Exec(ctx, `
			UPDATE transcode_jobs
			SET status = 'SUCCEEDED', attempt_count = GREATEST(attempt_count, $2),
				finished_at = $3, updated_at = $3, error_code = NULL
			WHERE id = $1`, result.JobID, result.Attempt, now,
		); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE audios
			SET status = 'READY', duration_ms = $2, playback_key = $3,
				waveform_key = $4, updated_at = $5
			WHERE id = $1`,
			result.AudioID, result.DurationMS, result.Artifacts.PlaybackKey,
			result.Artifacts.WaveformKey, now,
		); err != nil {
			return err
		}
	} else {
		jobStatus := "FAILED"
		audioStatus := audio.StatusFailed
		if result.Error.Retryable && result.Attempt < maximumAttempts {
			jobStatus = "QUEUED"
			audioStatus = audio.StatusQueued
		}
		if _, err := tx.Exec(ctx, `
			UPDATE transcode_jobs
			SET status = $2, attempt_count = GREATEST(attempt_count, $3),
				error_code = $4, finished_at = CASE WHEN $2 = 'FAILED' THEN $5 ELSE NULL END,
				updated_at = $5
			WHERE id = $1`,
			result.JobID, jobStatus, result.Attempt, result.Error.Code, now,
		); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE audios SET status = $2, updated_at = $3 WHERE id = $1`,
			result.AudioID, audioStatus, now,
		); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *Repository) ListUnpublishedOutbox(ctx context.Context, limit int) ([]events.OutboxEvent, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, event_type, schema_version, payload
		FROM outbox_events
		WHERE published_at IS NULL
		ORDER BY created_at
		LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]events.OutboxEvent, 0, limit)
	for rows.Next() {
		var event events.OutboxEvent
		if err := rows.Scan(&event.ID, &event.EventType, &event.SchemaVersion, &event.Payload); err != nil {
			return nil, err
		}
		result = append(result, event)
	}
	return result, rows.Err()
}

func (r *Repository) MarkOutboxPublished(ctx context.Context, eventID string, now time.Time) error {
	commandTag, err := r.pool.Exec(ctx, `
		UPDATE outbox_events SET published_at = $2
		WHERE id = $1 AND published_at IS NULL`, eventID, now)
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() == 0 {
		return fmt.Errorf("outbox event %s is not pending", eventID)
	}
	return nil
}

func registerProcessedEvent(ctx context.Context, tx pgx.Tx, eventID, eventType string, now time.Time) (bool, error) {
	commandTag, err := tx.Exec(ctx, `
		INSERT INTO processed_events (event_id, event_type, processed_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (event_id) DO NOTHING`, eventID, eventType, now)
	if err != nil {
		return false, err
	}
	return commandTag.RowsAffected() == 1, nil
}
