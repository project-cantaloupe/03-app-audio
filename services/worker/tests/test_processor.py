import base64
import hashlib

import pytest

from audio_worker.contracts import TranscodeJob
from audio_worker.processor import Processor, ProcessorConfig, WorkerError, result_event


def job(checksum: str = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="):
    return TranscodeJob.from_dict(
        {
            "schema_version": 1,
            "event_id": "018f86f3-2a5a-7f67-a1af-12655a22694e",
            "job_id": "018f86f3-2a5a-7f67-a1af-12655a22694f",
            "audio_id": "018f86f3-2a5a-7f67-a1af-12655a226950",
            "source": {
                "bucket": "cntlp-aws-quarantine",
                "key": "incoming/audio/upload/source",
                "version_id": "v1",
                "checksum_sha256": checksum,
            },
            "targets": {"mp3_bitrate_kbps": 192, "waveform_points_per_second": 20},
        }
    )


def processor():
    return Processor(object(), ProcessorConfig(artifact_bucket="cntlp-aws-transcode"))


def digest_of(payload: bytes) -> str:
    return base64.b64encode(hashlib.sha256(payload).digest()).decode()


# API의 Presigned URL이 체크섬을 서명하지 못해 S3에 SHA-256이 남지 않는다.
# 무결성 대조가 여기로 옮겨왔으므로 이 검증이 실제로 동작해야 한다.
def test_verify_checksum_accepts_matching_source(tmp_path):
    payload = b"cantaloupe source bytes" * 100
    source = tmp_path / "source"
    source.write_bytes(payload)

    processor()._verify_checksum(source, job(digest_of(payload)))


def test_verify_checksum_rejects_altered_source(tmp_path):
    source = tmp_path / "source"
    source.write_bytes(b"tampered")

    with pytest.raises(WorkerError) as error:
        processor()._verify_checksum(source, job(digest_of(b"original")))

    assert error.value.code == "SOURCE_CHECKSUM_MISMATCH"
    assert error.value.retryable is False


# 1 MiB 청크 경계를 넘는 파일에서도 스트리밍 해시가 맞아야 한다.
def test_verify_checksum_handles_multi_chunk_source(tmp_path):
    payload = b"x" * (1024 * 1024 * 2 + 7)
    source = tmp_path / "source"
    source.write_bytes(payload)

    processor()._verify_checksum(source, job(digest_of(payload)))


def test_result_event_id_is_stable_for_sqs_redelivery():
    first = result_event(job(), 2, "FAILED", error_code="TEST_FAILURE", retryable=True)
    second = result_event(job(), 2, "FAILED", error_code="TEST_FAILURE", retryable=True)

    assert first["event_id"] == second["event_id"]
    assert first["error"] == {"code": "TEST_FAILURE", "retryable": True}
