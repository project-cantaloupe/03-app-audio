import json

from audio_worker.contracts import TranscodeJob
from audio_worker.main import Config, Worker
from audio_worker.processor import ProcessedTranscode, result_event

TRANSCODE_PAYLOAD = {
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


class FakeSQS:
    def __init__(self) -> None:
        self.actions: list[tuple[str, dict[str, object]]] = []

    def send_message(self, **arguments: object) -> None:
        self.actions.append(("send", arguments))

    def delete_message(self, **arguments: object) -> None:
        self.actions.append(("delete", arguments))


class FakeProcessor:
    def __init__(self, processed: ProcessedTranscode) -> None:
        self.processed = processed
        self.calls: list[tuple[str, int]] = []

    def process(self, job: TranscodeJob, attempt: int) -> ProcessedTranscode:
        self.calls.append((job.job_id, attempt))
        return self.processed


class FakeHeartbeat:
    def __init__(self, *_: object) -> None:
        self.started = False
        self.stopped = False

    def start(self) -> None:
        self.started = True

    def stop(self) -> None:
        self.stopped = True


def config() -> Config:
    return Config(
        aws_region="ap-northeast-2",
        transcode_queue_url="https://sqs.example/transcode",
        result_queue_url="https://sqs.example/result",
        artifact_bucket="cntlp-aws-transcode",
        poll_wait_seconds=20,
        visibility_timeout_seconds=900,
        maximum_attempts=3,
        maximum_duration_seconds=7200,
        metrics_port=9090,
    )


def test_success_publishes_result_before_deleting_source_message(monkeypatch):
    job = TranscodeJob.from_dict(TRANSCODE_PAYLOAD)
    event = result_event(
        job,
        attempt=1,
        status="SUCCEEDED",
        duration_ms=3210,
        artifacts={
            "bucket": "cntlp-aws-transcode",
            "playback_key": "audios/audio/artifacts/job/playback.mp3",
            "waveform_key": "audios/audio/artifacts/job/waveform.json",
        },
    )
    processor = FakeProcessor(ProcessedTranscode(event=event, input_bytes=100, output_bytes=50))
    sqs = FakeSQS()
    monkeypatch.setattr("audio_worker.main.VisibilityHeartbeat", FakeHeartbeat)

    Worker(sqs, processor, config())._handle(
        {
            "ReceiptHandle": "receipt-1",
            "Attributes": {"ApproximateReceiveCount": "1"},
            "Body": json.dumps(TRANSCODE_PAYLOAD),
        }
    )

    assert processor.calls == [(job.job_id, 1)]
    assert [action for action, _ in sqs.actions] == ["send", "delete"]
    assert json.loads(sqs.actions[0][1]["MessageBody"]) == event
    assert sqs.actions[0][1]["QueueUrl"] == config().result_queue_url
    assert sqs.actions[1][1] == {
        "QueueUrl": config().transcode_queue_url,
        "ReceiptHandle": "receipt-1",
    }
