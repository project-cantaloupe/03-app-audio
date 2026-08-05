from audio_worker.contracts import TranscodeJob
from audio_worker.processor import result_event


def job():
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
                "checksum_sha256": "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
            },
            "targets": {"mp3_bitrate_kbps": 192, "waveform_points_per_second": 20},
        }
    )


def test_result_event_id_is_stable_for_sqs_redelivery():
    first = result_event(job(), 2, "FAILED", error_code="TEST_FAILURE", retryable=True)
    second = result_event(job(), 2, "FAILED", error_code="TEST_FAILURE", retryable=True)

    assert first["event_id"] == second["event_id"]
    assert first["error"] == {"code": "TEST_FAILURE", "retryable": True}
