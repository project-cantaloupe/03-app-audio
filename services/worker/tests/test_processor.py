import base64
import hashlib
from pathlib import Path

import pytest

from audio_worker import processor as processor_module
from audio_worker.contracts import TranscodeJob
from audio_worker.media import MediaInfo
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


class FakeS3:
    def __init__(self, source: bytes) -> None:
        self.source = source
        self.uploads: dict[tuple[str, str], dict[str, object]] = {}
        self.local_paths: list[Path] = []

    def get_object_tagging(self, **_: object) -> dict[str, object]:
        return {"TagSet": [{"Key": "CntlpScanStatus", "Value": "NO_THREATS_FOUND"}]}

    def download_file(self, _: str, __: str, filename: str, **___: object) -> None:
        Path(filename).write_bytes(self.source)

    def upload_file(
        self,
        filename: str,
        bucket: str,
        key: str,
        **options: object,
    ) -> None:
        path = Path(filename)
        self.local_paths.append(path)
        self.uploads[(bucket, key)] = {
            "payload": path.read_bytes(),
            "extra_args": options["ExtraArgs"],
        }


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


def test_process_returns_success_after_temporary_directory_cleanup(monkeypatch):
    source = b"cantaloupe source"
    playback = b"encoded mp3"
    waveform = b'{"schema_version":1,"peaks":[]}'
    s3 = FakeS3(source)

    monkeypatch.setattr(
        processor_module,
        "probe",
        lambda *_: MediaInfo(duration_ms=3210, codec_name="pcm_s16le"),
    )
    monkeypatch.setattr(
        processor_module,
        "transcode_mp3",
        lambda _, destination, __: destination.write_bytes(playback),
    )
    monkeypatch.setattr(
        processor_module,
        "write_waveform",
        lambda _, destination, __, ___: destination.write_bytes(waveform),
    )

    processed = Processor(
        s3,
        ProcessorConfig(artifact_bucket="cntlp-aws-transcode"),
    ).process(job(digest_of(source)), attempt=1)

    assert processed.input_bytes == len(source)
    assert processed.output_bytes == len(playback)
    assert processed.event["status"] == "SUCCEEDED"
    assert processed.event["duration_ms"] == 3210
    assert processed.event["artifacts"] == {
        "bucket": "cntlp-aws-transcode",
        "playback_key": (
            "audios/018f86f3-2a5a-7f67-a1af-12655a226950/artifacts/"
            "018f86f3-2a5a-7f67-a1af-12655a22694f/playback.mp3"
        ),
        "waveform_key": (
            "audios/018f86f3-2a5a-7f67-a1af-12655a226950/artifacts/"
            "018f86f3-2a5a-7f67-a1af-12655a22694f/waveform.json"
        ),
    }
    assert (
        s3.uploads[("cntlp-aws-transcode", processed.event["artifacts"]["playback_key"])]["payload"]
        == playback
    )
    assert (
        s3.uploads[("cntlp-aws-transcode", processed.event["artifacts"]["waveform_key"])]["payload"]
        == waveform
    )
    assert all(not path.exists() for path in s3.local_paths)


def test_result_event_id_is_stable_for_sqs_redelivery():
    first = result_event(job(), 2, "FAILED", error_code="TEST_FAILURE", retryable=True)
    second = result_event(job(), 2, "FAILED", error_code="TEST_FAILURE", retryable=True)

    assert first["event_id"] == second["event_id"]
    assert first["error"] == {"code": "TEST_FAILURE", "retryable": True}
