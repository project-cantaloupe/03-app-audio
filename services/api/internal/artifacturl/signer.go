package artifacturl

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cloudfrontsign "github.com/aws/aws-sdk-go-v2/feature/cloudfront/sign"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3Signer struct {
	presigner *s3.PresignClient
	bucket    string
}

func NewS3Signer(client *s3.Client, bucket string) *S3Signer {
	return &S3Signer{presigner: s3.NewPresignClient(client), bucket: bucket}
}

func (s *S3Signer) Sign(ctx context.Context, key string, expiresAt time.Time) (string, error) {
	expiry := time.Until(expiresAt)
	if expiry <= 0 {
		return "", errors.New("artifact URL expiry must be in the future")
	}
	result, err := s.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, func(options *s3.PresignOptions) {
		options.Expires = expiry
	})
	if err != nil {
		return "", err
	}
	return result.URL, nil
}

type CloudFrontSigner struct {
	baseURL string
	signer  *cloudfrontsign.URLSigner
}

func NewCloudFrontSigner(baseURL, keyPairID, privateKeyFile string) (*CloudFrontSigner, error) {
	parsed, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("CLOUDFRONT_BASE_URL must be an HTTPS origin without a query or fragment")
	}
	if strings.TrimSpace(keyPairID) == "" || strings.TrimSpace(privateKeyFile) == "" {
		return nil, errors.New("CloudFront key pair ID and private key file are required")
	}
	privateKey, err := cloudfrontsign.LoadPEMPrivKeyFile(privateKeyFile)
	if err != nil {
		return nil, fmt.Errorf("load CloudFront private key: %w", err)
	}
	signer := cloudfrontsign.NewURLSigner(keyPairID, privateKey)
	signer.HashAlg = cloudfrontsign.HashSHA256
	return &CloudFrontSigner{baseURL: parsed.String(), signer: signer}, nil
}

func (s *CloudFrontSigner) Sign(ctx context.Context, key string, expiresAt time.Time) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	resource, err := url.JoinPath(s.baseURL, key)
	if err != nil {
		return "", fmt.Errorf("build CloudFront artifact URL: %w", err)
	}
	return s.signer.Sign(resource, expiresAt)
}
