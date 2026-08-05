package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/project-cantaloupe/app-audio/services/api/internal/artifacturl"
	"github.com/project-cantaloupe/app-audio/services/api/internal/audio"
	"github.com/project-cantaloupe/app-audio/services/api/internal/health"
	"github.com/project-cantaloupe/app-audio/services/api/internal/httpapi"
	"github.com/project-cantaloupe/app-audio/services/api/internal/platform"
	"github.com/project-cantaloupe/app-audio/services/api/internal/postgres"
	"github.com/project-cantaloupe/app-audio/services/api/internal/s3store"
	"github.com/project-cantaloupe/app-audio/services/api/internal/scanadapter"
)

func main() {
	logger := log.New(os.Stdout, "audio-api ", log.LstdFlags|log.LUTC)
	if err := run(context.Background(), logger); err != nil {
		logger.Fatal(err)
	}
}

func run(parent context.Context, logger *log.Logger) error {
	config, err := loadConfig()
	if err != nil {
		return err
	}
	if config.AuthMode != "development" {
		return errors.New("only AUTH_MODE=development is implemented; Cognito verification must be added before public deployment")
	}
	logger.Print("warning: development subject header authentication is enabled")
	logger.Print("warning: development scan adapter marks verified uploads clean without malware inspection")

	ctx, cancel := signal.NotifyContext(parent, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	pool, err := pgxpool.New(ctx, config.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open PostgreSQL pool: %w", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping PostgreSQL: %w", err)
	}

	awsOptions := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(config.AWSRegion)}
	awsConfig, err := awsconfig.LoadDefaultConfig(ctx, awsOptions...)
	if err != nil {
		return fmt.Errorf("load AWS configuration: %w", err)
	}
	s3Client := s3.NewFromConfig(awsConfig, func(options *s3.Options) {
		if config.AWSEndpointURL != "" {
			options.BaseEndpoint = aws.String(config.AWSEndpointURL)
			options.UsePathStyle = true
		}
	})
	sqsClient := sqs.NewFromConfig(awsConfig, func(options *sqs.Options) {
		if config.AWSEndpointURL != "" {
			options.BaseEndpoint = aws.String(config.AWSEndpointURL)
		}
	})
	presignClient := s3Client
	if config.S3PublicEndpointURL != "" {
		presignClient = s3.NewFromConfig(awsConfig, func(options *s3.Options) {
			options.BaseEndpoint = aws.String(config.S3PublicEndpointURL)
			options.UsePathStyle = true
		})
	}

	repository := postgres.New(pool)
	objectStore := s3store.New(s3Client, presignClient)
	var artifactURLs audio.ArtifactURLSigner
	switch config.PlaybackURLMode {
	case "s3":
		artifactURLs = artifacturl.NewS3Signer(presignClient, config.ArtifactBucket)
	case "cloudfront":
		artifactURLs, err = artifacturl.NewCloudFrontSigner(
			config.CloudFrontBaseURL,
			config.CloudFrontKeyPairID,
			config.CloudFrontPrivateKeyFile,
		)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported PLAYBACK_URL_MODE %q", config.PlaybackURLMode)
	}
	service := audio.NewService(
		repository,
		objectStore,
		scanadapter.NewDevelopment(s3Client, sqsClient, config.ScanResultQueueURL),
		artifactURLs,
		platform.UUIDGenerator{},
		platform.SystemClock{},
		config.QuarantineBucket,
		config.UploadURLTTL,
		config.PlaybackURLTTL,
	)
	probe := &health.Probe{}
	probe.SetReady(true)
	server := &http.Server{
		Addr:              config.HTTPAddress,
		Handler:           httpapi.New(service, probe, logger),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errChannel := make(chan error, 1)
	go func() {
		logger.Printf("listening address=%s", config.HTTPAddress)
		errChannel <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		probe.SetReady(false)
		shutdownContext, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		return server.Shutdown(shutdownContext)
	case err := <-errChannel:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

type config struct {
	HTTPAddress              string
	DatabaseURL              string
	AWSRegion                string
	AWSEndpointURL           string
	S3PublicEndpointURL      string
	QuarantineBucket         string
	ArtifactBucket           string
	UploadURLTTL             time.Duration
	PlaybackURLTTL           time.Duration
	PlaybackURLMode          string
	CloudFrontBaseURL        string
	CloudFrontKeyPairID      string
	CloudFrontPrivateKeyFile string
	ScanResultQueueURL       string
	AuthMode                 string
}

func loadConfig() (config, error) {
	ttlSeconds, err := strconv.Atoi(valueOrDefault("UPLOAD_URL_TTL_SECONDS", "900"))
	if err != nil || ttlSeconds < 60 || ttlSeconds > 3600 {
		return config{}, errors.New("UPLOAD_URL_TTL_SECONDS must be between 60 and 3600")
	}
	playbackTTLSeconds, err := strconv.Atoi(valueOrDefault("PLAYBACK_URL_TTL_SECONDS", "10800"))
	if err != nil || playbackTTLSeconds < 300 || playbackTTLSeconds > 86400 {
		return config{}, errors.New("PLAYBACK_URL_TTL_SECONDS must be between 300 and 86400")
	}
	result := config{
		HTTPAddress:              valueOrDefault("HTTP_ADDRESS", ":8080"),
		DatabaseURL:              os.Getenv("DATABASE_URL"),
		AWSRegion:                os.Getenv("AWS_REGION"),
		AWSEndpointURL:           os.Getenv("AWS_ENDPOINT_URL"),
		S3PublicEndpointURL:      os.Getenv("S3_PUBLIC_ENDPOINT_URL"),
		QuarantineBucket:         os.Getenv("QUARANTINE_BUCKET"),
		ArtifactBucket:           os.Getenv("ARTIFACT_BUCKET"),
		UploadURLTTL:             time.Duration(ttlSeconds) * time.Second,
		PlaybackURLTTL:           time.Duration(playbackTTLSeconds) * time.Second,
		PlaybackURLMode:          valueOrDefault("PLAYBACK_URL_MODE", "s3"),
		CloudFrontBaseURL:        os.Getenv("CLOUDFRONT_BASE_URL"),
		CloudFrontKeyPairID:      os.Getenv("CLOUDFRONT_KEY_PAIR_ID"),
		CloudFrontPrivateKeyFile: os.Getenv("CLOUDFRONT_PRIVATE_KEY_FILE"),
		ScanResultQueueURL:       os.Getenv("SCAN_RESULT_QUEUE_URL"),
		AuthMode:                 os.Getenv("AUTH_MODE"),
	}
	if result.DatabaseURL == "" || result.AWSRegion == "" || result.QuarantineBucket == "" || result.ArtifactBucket == "" || result.ScanResultQueueURL == "" {
		return config{}, errors.New("DATABASE_URL, AWS_REGION, QUARANTINE_BUCKET, ARTIFACT_BUCKET, and SCAN_RESULT_QUEUE_URL are required")
	}
	if result.PlaybackURLMode == "cloudfront" && (result.CloudFrontBaseURL == "" || result.CloudFrontKeyPairID == "" || result.CloudFrontPrivateKeyFile == "") {
		return config{}, errors.New("CloudFront playback mode requires CLOUDFRONT_BASE_URL, CLOUDFRONT_KEY_PAIR_ID, and CLOUDFRONT_PRIVATE_KEY_FILE")
	}
	return result, nil
}

func valueOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
