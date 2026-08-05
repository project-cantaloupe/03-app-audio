from prometheus_client import Counter, Gauge, Histogram

COMPLETED = Counter(
    "audio_transcode_completed_total",
    "정상 완료된 오디오 트랜스코딩 작업 수",
)
FAILED = Counter(
    "audio_transcode_failed_total",
    "실패한 오디오 트랜스코딩 시도 수",
    labelnames=("error_code", "retryable"),
)
RETRIED = Counter(
    "audio_transcode_retried_total",
    "SQS에서 재수신한 트랜스코딩 작업 수",
)
DURATION = Histogram(
    "audio_transcode_duration_seconds",
    "오디오 트랜스코딩 작업 처리 시간",
    buckets=(1, 5, 10, 30, 60, 120, 300, 600, 1200, 2400),
)
IN_PROGRESS = Gauge(
    "audio_transcode_in_progress",
    "현재 Worker가 처리 중인 작업 수",
)
