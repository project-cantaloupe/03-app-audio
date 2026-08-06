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

// 체크섬을 Presign에 넣지 않는다.
//
// SigV4 Presigner는 X-Amz-* 헤더를 쿼리스트링으로 hoisting한다. 그 결과
// SignedHeaders에는 content-length, content-type, host만 남고 체크섬은 쿼리로
// 빠진다. 클라이언트가 x-amz-checksum-sha256을 헤더로 보내면 서명되지 않은
// 헤더라며 S3가 403을 반환하고, 보내지 않으면 S3는 쿼리의 값을 무시한 채
// 자체 기본 체크섬만 기록한다. 어느 쪽이든 SHA-256이 저장되지 않는다.
//
// 따라서 업로드 시점에는 크기와 Content-Type만 강제하고, SHA-256 대조는
// 원본을 실제로 내려받는 transcode 워커가 수행한다.
func (s *Store) PresignPut(ctx context.Context, record audio.Audio, expiry time.Duration) (string, error) {
	result, err := s.presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(record.SourceBucket),
		Key:           aws.String(record.SourceKey),
		ContentLength: aws.Int64(record.SourceSize),
		ContentType:   aws.String(record.SourceContentType),
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
