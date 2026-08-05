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
	"sync/atomic"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/project-cantaloupe/app-audio/services/api/internal/platform"
	"github.com/project-cantaloupe/app-audio/services/api/internal/postgres"
	"github.com/project-cantaloupe/app-audio/services/api/internal/sqsevents"
)

func main() {
	logger := log.New(os.Stdout, "audio-events ", log.LstdFlags|log.LUTC)
	if err := run(context.Background(), logger); err != nil {
		logger.Fatal(err)
	}
}

func run(parent context.Context, logger *log.Logger) error {
	config, err := loadConfig()
	if err != nil {
		return err
	}
	ctx, cancelSignal := signal.NotifyContext(parent, syscall.SIGINT, syscall.SIGTERM)
	defer cancelSignal()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	pool, err := pgxpool.New(ctx, config.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open PostgreSQL pool: %w", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping PostgreSQL: %w", err)
	}

	awsConfig, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(config.AWSRegion))
	if err != nil {
		return fmt.Errorf("load AWS configuration: %w", err)
	}
	sqsClient := sqs.NewFromConfig(awsConfig, func(options *sqs.Options) {
		if config.AWSEndpointURL != "" {
			options.BaseEndpoint = aws.String(config.AWSEndpointURL)
		}
	})

	repository := postgres.New(pool)
	runner := sqsevents.New(
		sqsClient,
		repository,
		platform.UUIDGenerator{},
		platform.SystemClock{},
		config.ScanQueueURL,
		config.ResultQueueURL,
		config.TranscodeQueueURL,
		config.MaximumAttempts,
		logger,
	)

	var ready atomic.Bool
	ready.Store(true)
	server := healthServer(config.HTTPAddress, &ready)
	errorChannel := make(chan error, 4)
	go func() { errorChannel <- server.ListenAndServe() }()
	go func() { errorChannel <- runner.ConsumeScanResults(ctx) }()
	go func() { errorChannel <- runner.ConsumeTranscodeResults(ctx) }()
	go func() { errorChannel <- runner.PublishOutbox(ctx) }()
	logger.Printf("started address=%s", config.HTTPAddress)

	select {
	case <-ctx.Done():
	case err := <-errorChannel:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			cancel()
			return err
		}
	}
	ready.Store(false)
	shutdownContext, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	return server.Shutdown(shutdownContext)
}

func healthServer(address string, ready *atomic.Bool) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if !ready.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"not_ready"}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"ready"}`))
	})
	return &http.Server{
		Addr:              address,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

type config struct {
	HTTPAddress       string
	DatabaseURL       string
	AWSRegion         string
	AWSEndpointURL    string
	ScanQueueURL      string
	ResultQueueURL    string
	TranscodeQueueURL string
	MaximumAttempts   int
}

func loadConfig() (config, error) {
	attempts, err := strconv.Atoi(valueOrDefault("TRANSCODE_MAXIMUM_ATTEMPTS", "3"))
	if err != nil || attempts < 1 || attempts > 20 {
		return config{}, errors.New("TRANSCODE_MAXIMUM_ATTEMPTS must be between 1 and 20")
	}
	result := config{
		HTTPAddress:       valueOrDefault("HTTP_ADDRESS", ":8081"),
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		AWSRegion:         os.Getenv("AWS_REGION"),
		AWSEndpointURL:    os.Getenv("AWS_ENDPOINT_URL"),
		ScanQueueURL:      os.Getenv("SCAN_RESULT_QUEUE_URL"),
		ResultQueueURL:    os.Getenv("TRANSCODE_RESULT_QUEUE_URL"),
		TranscodeQueueURL: os.Getenv("TRANSCODE_QUEUE_URL"),
		MaximumAttempts:   attempts,
	}
	if result.DatabaseURL == "" || result.AWSRegion == "" || result.ScanQueueURL == "" ||
		result.ResultQueueURL == "" || result.TranscodeQueueURL == "" {
		return config{}, errors.New("DATABASE_URL, AWS_REGION, and all three queue URLs are required")
	}
	return result, nil
}

func valueOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
