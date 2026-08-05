package s3store

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/project-cantaloupe/app-audio/services/api/internal/audio"
)

type Store struct {
	client    *s3.Client
	presigner *s3.PresignClient
}

func New(client, presignClient *s3.Client) *Store {
	return &Store{client: client, presigner: s3.NewPresignClient(presignClient)}
}

func (s *Store) PresignPut(ctx context.Context, record audio.Audio, expiry time.Duration) (string, error) {
	result, err := s.presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:            aws.String(record.SourceBucket),
		Key:               aws.String(record.SourceKey),
		ContentLength:     aws.Int64(record.SourceSize),
		ContentType:       aws.String(record.SourceContentType),
		ChecksumAlgorithm: types.ChecksumAlgorithmSha256,
		ChecksumSHA256:    aws.String(record.SourceChecksum),
	}, func(options *s3.PresignOptions) {
		options.Expires = expiry
	})
	if err != nil {
		return "", err
	}
	return result.URL, nil
}

func (s *Store) HeadSource(ctx context.Context, record audio.Audio) (audio.SourceObject, error) {
	result, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket:       aws.String(record.SourceBucket),
		Key:          aws.String(record.SourceKey),
		ChecksumMode: types.ChecksumModeEnabled,
	})
	if err != nil {
		return audio.SourceObject{}, err
	}
	return audio.SourceObject{
		VersionID:      aws.ToString(result.VersionId),
		ContentLength:  aws.ToInt64(result.ContentLength),
		ContentType:    aws.ToString(result.ContentType),
		ChecksumSHA256: aws.ToString(result.ChecksumSHA256),
	}, nil
}
