import json
import logging

from audio_worker.log_format import JSONFormatter


def test_json_formatter_keeps_only_safe_finops_fields():
    record = logging.LogRecord(
        "audio-worker", logging.INFO, __file__, 0, "transcode completed", (), None
    )
    record.event_type = "transcode_completed"
    record.job_id = "job-123"
    record.input_bytes = 10
    record.source_key = "must-not-be-indexed"

    payload = json.loads(JSONFormatter().format(record))

    assert payload == {
        "event_type": "transcode_completed",
        "input_bytes": 10,
        "job_id": "job-123",
        "level": "info",
        "message": "transcode completed",
    }
