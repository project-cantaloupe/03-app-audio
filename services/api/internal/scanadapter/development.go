package scanadapter

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/project-cantaloupe/app-audio/services/api/internal/audio"
	"github.com/project-cantaloupe/app-audio/services/api/internal/events"
)

const CleanTagKey = "CntlpScanStatus"

type s3Tagger interface {
	PutObjectTagging(context.Context, *s3.PutObjectTaggingInput, ...func(*s3.Options)) (*s3.PutObjectTaggingOutput, error)
}

type sqsSender interface {
	SendMessage(context.Context, *sqs.SendMessageInput, ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
}

type Development struct {
	s3Client  s3Tagger
	sqsClient sqsSender
	queueURL  string
}

func NewDevelopment(s3Client s3Tagger, sqsClient sqsSender, queueURL string) *Development {
	return &Development{s3Client: s3Client, sqsClient: sqsClient, queueURL: queueURL}
}

func (d *Development) Submit(ctx context.Context, request audio.SourceScanRequest) error {
	result := events.ScanResult{
		SchemaVersion: 1,
		EventID:       request.EventID,
		Bucket:        request.Bucket,
		Key:           request.Key,
		VersionID:     request.VersionID,
		Status:        request.Status,
		OccurredAt:    request.OccurredAt,
	}
	if err := result.Validate(); err != nil {
		return fmt.Errorf("validate scan result: %w", err)
	}
	if result.Status != "NO_THREATS_FOUND" {
		return fmt.Errorf("development scan adapter only supports NO_THREATS_FOUND")
	}

	_, err := d.s3Client.PutObjectTagging(ctx, &s3.PutObjectTaggingInput{
		Bucket:    aws.String(result.Bucket),
		Key:       aws.String(result.Key),
		VersionId: aws.String(result.VersionID),
		Tagging: &types.Tagging{TagSet: []types.Tag{
			{Key: aws.String(CleanTagKey), Value: aws.String(result.Status)},
		}},
	})
	if err != nil {
		return fmt.Errorf("tag source version: %w", err)
	}

	body, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("encode scan result: %w", err)
	}
	if _, err := d.sqsClient.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(d.queueURL),
		MessageBody: aws.String(string(body)),
	}); err != nil {
		return fmt.Errorf("publish scan result: %w", err)
	}
	return nil
}
