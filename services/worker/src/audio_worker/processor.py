from __future__ import annotations

import json
import tempfile
import uuid
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

from .contracts import TranscodeJob
from .media import MediaError, probe, transcode_mp3, write_waveform


class WorkerError(RuntimeError):
    def __init__(self, code: str, retryable: bool) -> None:
        super().__init__(code)
        self.code = code
        self.retryable = retryable


@dataclass(frozen=True)
class ProcessorConfig:
    artifact_bucket: str
    maximum_duration_seconds: int = 7200
    clean_tag_key: str = "CntlpScanStatus"
    clean_tag_value: str = "NO_THREATS_FOUND"


class Processor:
    def __init__(self, s3_client: Any, config: ProcessorConfig) -> None:
        self._s3 = s3_client
        self._config = config

    def process(self, job: TranscodeJob, attempt: int) -> dict[str, Any]:
        self._verify_source(job)
        playback_key = f"audios/{job.audio_id}/artifacts/{job.job_id}/playback.mp3"
        waveform_key = f"audios/{job.audio_id}/artifacts/{job.job_id}/waveform.json"

        with tempfile.TemporaryDirectory(prefix="cntlp-audio-") as directory:
            workdir = Path(directory)
            source_path = workdir / "source"
            playback_path = workdir / "playback.mp3"
            waveform_path = workdir / "waveform.json"
            try:
                self._s3.download_file(
                    job.source.bucket,
                    job.source.key,
                    str(source_path),
                    ExtraArgs={"VersionId": job.source.version_id},
                )
            except Exception as error:
                raise WorkerError("SOURCE_DOWNLOAD_FAILED", retryable=True) from error

            try:
                media = probe(source_path, self._config.maximum_duration_seconds)
                transcode_mp3(source_path, playback_path, job.targets.mp3_bitrate_kbps)
                write_waveform(
                    source_path,
                    waveform_path,
                    media.duration_ms,
                    job.targets.waveform_points_per_second,
                )
            except MediaError as error:
                raise WorkerError("MEDIA_PROCESSING_FAILED", retryable=False) from error

            metadata = {"audio-id": job.audio_id, "job-id": job.job_id}
            try:
                self._s3.upload_file(
                    str(playback_path),
                    self._config.artifact_bucket,
                    playback_key,
                    ExtraArgs={"ContentType": "audio/mpeg", "Metadata": metadata},
                )
                self._s3.upload_file(
                    str(waveform_path),
                    self._config.artifact_bucket,
                    waveform_key,
                    ExtraArgs={"ContentType": "application/json", "Metadata": metadata},
                )
            except Exception as error:
                raise WorkerError("ARTIFACT_UPLOAD_FAILED", retryable=True) from error

        return result_event(
            job,
            attempt,
            status="SUCCEEDED",
            duration_ms=media.duration_ms,
            artifacts={
                "bucket": self._config.artifact_bucket,
                "playback_key": playback_key,
                "waveform_key": waveform_key,
            },
        )

    def _verify_source(self, job: TranscodeJob) -> None:
        try:
            response = self._s3.head_object(
                Bucket=job.source.bucket,
                Key=job.source.key,
                VersionId=job.source.version_id,
                ChecksumMode="ENABLED",
            )
            tags = self._s3.get_object_tagging(
                Bucket=job.source.bucket,
                Key=job.source.key,
                VersionId=job.source.version_id,
            )
        except Exception as error:
            raise WorkerError("SOURCE_METADATA_UNAVAILABLE", retryable=True) from error

        if response.get("ChecksumSHA256") != job.source.checksum_sha256:
            raise WorkerError("SOURCE_CHECKSUM_MISMATCH", retryable=False)
        tag_values = {item["Key"]: item["Value"] for item in tags.get("TagSet", [])}
        if tag_values.get(self._config.clean_tag_key) != self._config.clean_tag_value:
            raise WorkerError("SOURCE_NOT_CLEAN", retryable=False)


def result_event(
    job: TranscodeJob,
    attempt: int,
    status: str,
    duration_ms: int = 0,
    artifacts: dict[str, str] | None = None,
    error_code: str = "",
    retryable: bool = False,
) -> dict[str, Any]:
    event_id = uuid.uuid5(uuid.UUID(job.job_id), f"transcode-result:{status}:{attempt}")
    event: dict[str, Any] = {
        "schema_version": 1,
        "event_id": str(event_id),
        "job_id": job.job_id,
        "audio_id": job.audio_id,
        "status": status,
        "attempt": attempt,
        "occurred_at": datetime.now(timezone.utc).isoformat().replace("+00:00", "Z"),
    }
    if status == "SUCCEEDED":
        event["duration_ms"] = duration_ms
        event["artifacts"] = artifacts or {}
    else:
        event["error"] = {"code": error_code, "retryable": retryable}
    return event


def encode_result(event: dict[str, Any]) -> str:
    return json.dumps(event, separators=(",", ":"), sort_keys=True)
