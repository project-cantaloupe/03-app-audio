package scanadapter

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/project-cantaloupe/app-audio/services/api/internal/audio"
	"github.com/project-cantaloupe/app-audio/services/api/internal/events"
)

type fakeS3Tagger struct{ input *s3.PutObjectTaggingInput }

func (f *fakeS3Tagger) PutObjectTagging(_ context.Context, input *s3.PutObjectTaggingInput, _ ...func(*s3.Options)) (*s3.PutObjectTaggingOutput, error) {
	f.input = input
	return &s3.PutObjectTaggingOutput{}, nil
}

type fakeSQSSender struct{ input *sqs.SendMessageInput }

func (f *fakeSQSSender) SendMessage(_ context.Context, input *sqs.SendMessageInput, _ ...func(*sqs.Options)) (*sqs.SendMessageOutput, error) {
	f.input = input
	return &sqs.SendMessageOutput{}, nil
}

func TestDevelopmentTagsExactVersionAndPublishesContract(t *testing.T) {
	s3Client := &fakeS3Tagger{}
	sqsClient := &fakeSQSSender{}
	adapter := NewDevelopment(s3Client, sqsClient, "https://sqs.example.test/scan-result")
	now := time.Date(2026, 8, 5, 14, 0, 0, 0, time.UTC)

	err := adapter.Submit(context.Background(), audio.SourceScanRequest{
		EventID: "f4ad20fe-d6a2-4f90-a417-a00cc66df31d",
		Bucket:  "cntlp-aws-quarantine", Key: "incoming/audio/upload/source",
		VersionID: "version-1", Status: "NO_THREATS_FOUND", OccurredAt: now,
	})
	if err != nil {
		t.Fatalf("Submit() error = %v", err)
	}
	if aws.ToString(s3Client.input.VersionId) != "version-1" {
		t.Fatalf("unexpected tagged version: %s", aws.ToString(s3Client.input.VersionId))
	}
	tags := s3Client.input.Tagging.TagSet
	if len(tags) != 1 || aws.ToString(tags[0].Key) != CleanTagKey || aws.ToString(tags[0].Value) != "NO_THREATS_FOUND" {
		t.Fatalf("unexpected tags: %#v", tags)
	}
	if aws.ToString(sqsClient.input.QueueUrl) != "https://sqs.example.test/scan-result" {
		t.Fatalf("unexpected queue URL: %s", aws.ToString(sqsClient.input.QueueUrl))
	}
	var message events.ScanResult
	if err := json.Unmarshal([]byte(aws.ToString(sqsClient.input.MessageBody)), &message); err != nil {
		t.Fatalf("decode message: %v", err)
	}
	if err := message.Validate(); err != nil {
		t.Fatalf("published message is invalid: %v", err)
	}
	if message.EventID != "f4ad20fe-d6a2-4f90-a417-a00cc66df31d" || !message.OccurredAt.Equal(now) {
		t.Fatalf("unexpected message: %#v", message)
	}
}
