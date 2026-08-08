"""Small, dependency-free JSON formatter for stdout application logs."""

from __future__ import annotations

import json
import logging
from typing import Any


class JSONFormatter(logging.Formatter):
    """Emit only the FinOps fields that are safe and useful to index."""

    _fields = (
        "event_type",
        "status",
        "job_id",
        "audio_id",
        "attempt",
        "retry_count",
        "audio_duration_ms",
        "processing_duration_ms",
        "input_bytes",
        "output_bytes",
        "error_code",
        "retryable",
        "metrics_port",
    )

    def format(self, record: logging.LogRecord) -> str:
        payload: dict[str, Any] = {
            "level": record.levelname.lower(),
            "message": record.getMessage(),
        }
        for field in self._fields:
            value = getattr(record, field, None)
            if value is not None:
                payload[field] = value
        return json.dumps(payload, separators=(",", ":"), sort_keys=True)
