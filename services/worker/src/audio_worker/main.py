from __future__ import annotations

import json
import logging
import os
import signal
import threading
import time
from dataclasses import dataclass
from typing import Any

import boto3
from prometheus_client import start_http_server

from .contracts import ContractError, TranscodeJob
from .log_format import JSONFormatter
from .metrics import COMPLETED, DURATION, FAILED, IN_PROGRESS, RETRIED
from .processor import Processor, ProcessorConfig, WorkerError, encode_result, result_event

LOGGER = logging.getLogger("audio-worker")


@dataclass(frozen=True)
class Config:
    aws_region: str
    transcode_queue_url: str
    result_queue_url: str
    artifact_bucket: str
    poll_wait_seconds: int
    visibility_timeout_seconds: int
    maximum_attempts: int
    maximum_duration_seconds: int
    metrics_port: int

    @classmethod
    def from_environment(cls) -> Config:
        return cls(
            aws_region=_required("AWS_REGION"),
            transcode_queue_url=_required("TRANSCODE_QUEUE_URL"),
            result_queue_url=_required("TRANSCODE_RESULT_QUEUE_URL"),
            artifact_bucket=_required("ARTIFACT_BUCKET"),
            poll_wait_seconds=_integer("SQS_POLL_WAIT_SECONDS", 20, 1, 20),
            visibility_timeout_seconds=_integer("SQS_VISIBILITY_TIMEOUT_SECONDS", 900, 60, 43200),
            maximum_attempts=_integer("TRANSCODE_MAXIMUM_ATTEMPTS", 3, 1, 20),
            maximum_duration_seconds=_integer("AUDIO_MAXIMUM_DURATION_SECONDS", 7200, 1, 86400),
            metrics_port=_integer("METRICS_PORT", 9090, 1, 65535),
        )


class Worker:
    def __init__(self, sqs_client: Any, processor: Processor, config: Config) -> None:
        self._sqs = sqs_client
        self._processor = processor
        self._config = config
        self._stop = threading.Event()

    def stop(self, *_: Any) -> None:
        self._stop.set()

    def run(self) -> None:
        while not self._stop.is_set():
            response = self._sqs.receive_message(
                QueueUrl=self._config.transcode_queue_url,
                MaxNumberOfMessages=1,
                WaitTimeSeconds=self._config.poll_wait_seconds,
                VisibilityTimeout=self._config.visibility_timeout_seconds,
                AttributeNames=["ApproximateReceiveCount"],
            )
            for message in response.get("Messages", []):
                self._handle(message)

    def _handle(self, message: dict[str, Any]) -> None:
        receipt = message["ReceiptHandle"]
        attempt = int(message.get("Attributes", {}).get("ApproximateReceiveCount", "1"))
        if attempt > 1:
            RETRIED.inc()
        try:
            payload = json.loads(message["Body"])
            job = TranscodeJob.from_dict(payload)
        except (json.JSONDecodeError, ContractError, KeyError, TypeError) as error:
            FAILED.labels(error_code="INVALID_MESSAGE", retryable="false").inc()
            LOGGER.error(
                "invalid transcode message",
                extra={"error": str(error), "attempt": attempt},
            )
            return

        started = time.monotonic()
        IN_PROGRESS.inc()
        heartbeat = VisibilityHeartbeat(
            self._sqs,
            self._config.transcode_queue_url,
            receipt,
            self._config.visibility_timeout_seconds,
        )
        heartbeat.start()
        try:
            processed = self._processor.process(job, attempt)
            self._publish(processed.event)
            self._delete(receipt)
            COMPLETED.inc()
            LOGGER.info(
                "transcode completed",
                extra={
                    "event_type": "transcode_completed",
                    "status": "success",
                    "job_id": job.job_id,
                    "audio_id": job.audio_id,
                    "attempt": attempt,
                    "retry_count": max(attempt - 1, 0),
                    "audio_duration_ms": processed.event["duration_ms"],
                    "processing_duration_ms": round((time.monotonic() - started) * 1000),
                    "input_bytes": processed.input_bytes,
                    "output_bytes": processed.output_bytes,
                },
            )
        except WorkerError as error:
            FAILED.labels(error_code=error.code, retryable=str(error.retryable).lower()).inc()
            LOGGER.error(
                "transcode failed",
                extra={
                    "event_type": "transcode_failed",
                    "status": "failed",
                    "job_id": job.job_id,
                    "audio_id": job.audio_id,
                    "attempt": attempt,
                    "retry_count": max(attempt - 1, 0),
                    "error_code": error.code,
                    "retryable": error.retryable,
                },
            )
            final_attempt = attempt >= self._config.maximum_attempts
            if not error.retryable or final_attempt:
                self._publish(
                    result_event(
                        job,
                        attempt,
                        status="FAILED",
                        error_code=error.code,
                        retryable=error.retryable,
                    )
                )
            if not error.retryable:
                self._delete(receipt)
        except Exception:
            FAILED.labels(error_code="WORKER_INTERNAL_ERROR", retryable="true").inc()
            LOGGER.exception(
                "transcode attempt interrupted",
                extra={
                    "event_type": "transcode_failed",
                    "status": "failed",
                    "job_id": job.job_id,
                    "audio_id": job.audio_id,
                    "attempt": attempt,
                    "retry_count": max(attempt - 1, 0),
                    "error_code": "WORKER_INTERNAL_ERROR",
                    "retryable": True,
                },
            )
        finally:
            heartbeat.stop()
            IN_PROGRESS.dec()
            DURATION.observe(time.monotonic() - started)

    def _publish(self, event: dict[str, Any]) -> None:
        self._sqs.send_message(
            QueueUrl=self._config.result_queue_url,
            MessageBody=encode_result(event),
        )

    def _delete(self, receipt: str) -> None:
        self._sqs.delete_message(QueueUrl=self._config.transcode_queue_url, ReceiptHandle=receipt)


class VisibilityHeartbeat:
    def __init__(self, sqs_client: Any, queue_url: str, receipt: str, timeout: int) -> None:
        self._sqs = sqs_client
        self._queue_url = queue_url
        self._receipt = receipt
        self._timeout = timeout
        self._stop = threading.Event()
        self._thread: threading.Thread | None = None

    def start(self) -> None:
        self._thread = threading.Thread(target=self._run, daemon=True)
        self._thread.start()

    def stop(self) -> None:
        self._stop.set()
        if self._thread is not None:
            self._thread.join(timeout=2)

    def _run(self) -> None:
        interval = max(30, self._timeout // 2)
        while not self._stop.wait(interval):
            try:
                self._sqs.change_message_visibility(
                    QueueUrl=self._queue_url,
                    ReceiptHandle=self._receipt,
                    VisibilityTimeout=self._timeout,
                )
            except Exception:
                LOGGER.exception("could not extend SQS visibility timeout")


def main() -> None:
    handler = logging.StreamHandler()
    handler.setFormatter(JSONFormatter())
    LOGGER.setLevel(os.getenv("LOG_LEVEL", "INFO"))
    LOGGER.handlers.clear()
    LOGGER.addHandler(handler)
    LOGGER.propagate = False
    config = Config.from_environment()
    session = boto3.session.Session(region_name=config.aws_region)
    sqs = session.client("sqs", endpoint_url=os.getenv("AWS_ENDPOINT_URL") or None)
    s3 = session.client("s3", endpoint_url=os.getenv("AWS_ENDPOINT_URL") or None)
    processor = Processor(
        s3,
        ProcessorConfig(
            artifact_bucket=config.artifact_bucket,
            maximum_duration_seconds=config.maximum_duration_seconds,
        ),
    )
    worker = Worker(sqs, processor, config)
    signal.signal(signal.SIGTERM, worker.stop)
    signal.signal(signal.SIGINT, worker.stop)
    start_http_server(config.metrics_port)
    LOGGER.info(
        "audio worker started",
        extra={"event_type": "worker_started", "status": "ready", "metrics_port": config.metrics_port},
    )
    worker.run()


def _required(name: str) -> str:
    value = os.getenv(name, "").strip()
    if not value:
        raise ValueError(f"{name} is required")
    return value


def _integer(name: str, default: int, minimum: int, maximum: int) -> int:
    value = int(os.getenv(name, str(default)))
    if not minimum <= value <= maximum:
        raise ValueError(f"{name} must be between {minimum} and {maximum}")
    return value


if __name__ == "__main__":
    main()
