package sqsevents

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/project-cantaloupe/app-audio/services/api/internal/events"
	"github.com/project-cantaloupe/app-audio/services/api/internal/observability"
)

type Repository interface {
	ProcessScanResult(context.Context, events.ScanResult, string, string, time.Time) error
	ProcessTranscodeResult(context.Context, events.TranscodeResult, int, time.Time) error
	ListUnpublishedOutbox(context.Context, int) ([]events.OutboxEvent, error)
	MarkOutboxPublished(context.Context, string, time.Time) error
}

type IDGenerator interface{ New() string }
type Clock interface{ Now() time.Time }

type Runner struct {
	client            *sqs.Client
	repository        Repository
	ids               IDGenerator
	clock             Clock
	scanQueueURL      string
	resultQueueURL    string
	transcodeQueueURL string
	maximumAttempts   int
	logger            *log.Logger
}

func New(client *sqs.Client, repository Repository, ids IDGenerator, clock Clock, scanQueueURL, resultQueueURL, transcodeQueueURL string, maximumAttempts int, logger *log.Logger) *Runner {
	return &Runner{
		client: client, repository: repository, ids: ids, clock: clock,
		scanQueueURL: scanQueueURL, resultQueueURL: resultQueueURL,
		transcodeQueueURL: transcodeQueueURL, maximumAttempts: maximumAttempts,
		logger: logger,
	}
}

func (r *Runner) ConsumeScanResults(ctx context.Context) error {
	return r.consume(ctx, r.scanQueueURL, func(body []byte) error {
		var result events.ScanResult
		if err := decodeStrict(body, &result); err != nil {
			return err
		}
		if err := result.Validate(); err != nil {
			return err
		}
		err := r.repository.ProcessScanResult(
			ctx, result, r.ids.New(), r.ids.New(), r.clock.Now().UTC(),
		)
		if err == nil {
			observability.Event(r.logger, "info", "scan_result_processed", "scan result processed", map[string]any{
				"status": result.Status,
			})
		}
		return err
	})
}

func (r *Runner) ConsumeTranscodeResults(ctx context.Context) error {
	return r.consume(ctx, r.resultQueueURL, func(body []byte) error {
		var result events.TranscodeResult
		if err := decodeStrict(body, &result); err != nil {
			return err
		}
		if err := result.Validate(); err != nil {
			return err
		}
		err := r.repository.ProcessTranscodeResult(
			ctx, result, r.maximumAttempts, r.clock.Now().UTC(),
		)
		if err == nil {
			fields := map[string]any{
				"job_id": result.JobID, "audio_id": result.AudioID,
				"status": result.Status, "attempt": result.Attempt,
				"retry_count": max(result.Attempt-1, 0),
			}
			if result.Status == "SUCCEEDED" {
				fields["audio_duration_ms"] = result.DurationMS
			} else if result.Error != nil {
				fields["error_code"] = result.Error.Code
				fields["retryable"] = result.Error.Retryable
			}
			observability.Event(r.logger, "info", "transcode_result_processed", "transcode result processed", fields)
		}
		return err
	})
}

func (r *Runner) PublishOutbox(ctx context.Context) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		if err := r.publishBatch(ctx); err != nil {
			r.logger.Printf("outbox publish failed error=%q", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (r *Runner) publishBatch(ctx context.Context) error {
	pending, err := r.repository.ListUnpublishedOutbox(ctx, 20)
	if err != nil {
		return err
	}
	for _, event := range pending {
		if event.EventType != "TranscodeRequested" {
			return fmt.Errorf("unsupported outbox event type %q", event.EventType)
		}
		if _, err := r.client.SendMessage(ctx, &sqs.SendMessageInput{
			QueueUrl:    aws.String(r.transcodeQueueURL),
			MessageBody: aws.String(string(event.Payload)),
		}); err != nil {
			return err
		}
		if err := r.repository.MarkOutboxPublished(ctx, event.ID, r.clock.Now().UTC()); err != nil {
			return err
		}
	}
	return nil
}

func (r *Runner) consume(ctx context.Context, queueURL string, handle func([]byte) error) error {
	for {
		if ctx.Err() != nil {
			return nil
		}
		result, err := r.client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(queueURL),
			MaxNumberOfMessages: 1,
			WaitTimeSeconds:     20,
			VisibilityTimeout:   60,
		})
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}
		for _, message := range result.Messages {
			if err := handle([]byte(aws.ToString(message.Body))); err != nil {
				r.logger.Printf("queue message rejected queue=%s error=%q", queueURL, err)
				continue
			}
			if _, err := r.client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
				QueueUrl: aws.String(queueURL), ReceiptHandle: message.ReceiptHandle,
			}); err != nil {
				return err
			}
		}
	}
}

func decodeStrict(value []byte, destination any) error {
	decoder := json.NewDecoder(bytes.NewReader(value))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("message must contain exactly one JSON object")
	}
	return nil
}
