from __future__ import annotations

import base64
import binascii
import uuid
from dataclasses import dataclass
from typing import Any


class ContractError(ValueError):
    """Queue 메시지가 합의된 계약을 위반했을 때 발생한다."""


@dataclass(frozen=True)
class Source:
    bucket: str
    key: str
    version_id: str
    checksum_sha256: str


@dataclass(frozen=True)
class Targets:
    mp3_bitrate_kbps: int
    waveform_points_per_second: int


@dataclass(frozen=True)
class TranscodeJob:
    schema_version: int
    event_id: str
    job_id: str
    audio_id: str
    source: Source
    targets: Targets

    @classmethod
    def from_dict(cls, value: dict[str, Any]) -> TranscodeJob:
        expected = {"schema_version", "event_id", "job_id", "audio_id", "source", "targets"}
        _require_exact_keys(value, expected, "job")
        if value["schema_version"] != 1:
            raise ContractError("unsupported schema_version")
        for field in ("event_id", "job_id", "audio_id"):
            _require_uuid(value[field], field)

        source_value = _require_dict(value["source"], "source")
        _require_exact_keys(
            source_value,
            {"bucket", "key", "version_id", "checksum_sha256"},
            "source",
        )
        for field in ("bucket", "key", "version_id", "checksum_sha256"):
            _require_non_empty_string(source_value[field], f"source.{field}")
        _require_sha256(source_value["checksum_sha256"])

        target_value = _require_dict(value["targets"], "targets")
        _require_exact_keys(
            target_value,
            {"mp3_bitrate_kbps", "waveform_points_per_second"},
            "targets",
        )
        bitrate = target_value["mp3_bitrate_kbps"]
        points = target_value["waveform_points_per_second"]
        if bitrate not in (128, 192, 256, 320):
            raise ContractError("targets.mp3_bitrate_kbps is not supported")
        if not isinstance(points, int) or isinstance(points, bool) or not 1 <= points <= 100:
            raise ContractError("targets.waveform_points_per_second must be between 1 and 100")

        return cls(
            schema_version=1,
            event_id=value["event_id"],
            job_id=value["job_id"],
            audio_id=value["audio_id"],
            source=Source(**source_value),
            targets=Targets(
                mp3_bitrate_kbps=bitrate,
                waveform_points_per_second=points,
            ),
        )


def _require_dict(value: Any, field: str) -> dict[str, Any]:
    if not isinstance(value, dict):
        raise ContractError(f"{field} must be an object")
    return value


def _require_exact_keys(value: dict[str, Any], expected: set, field: str) -> None:
    actual = set(value)
    if actual != expected:
        missing = sorted(expected - actual)
        unknown = sorted(actual - expected)
        raise ContractError(
            f"{field} keys do not match contract: missing={missing}, unknown={unknown}"
        )


def _require_non_empty_string(value: Any, field: str) -> None:
    if not isinstance(value, str) or not value:
        raise ContractError(f"{field} must be a non-empty string")


def _require_uuid(value: Any, field: str) -> None:
    _require_non_empty_string(value, field)
    try:
        uuid.UUID(value)
    except (ValueError, AttributeError) as error:
        raise ContractError(f"{field} must be a UUID") from error


def _require_sha256(value: Any) -> None:
    _require_non_empty_string(value, "source.checksum_sha256")
    try:
        decoded = base64.b64decode(value, validate=True)
    except (binascii.Error, ValueError) as error:
        raise ContractError("source.checksum_sha256 must be valid base64") from error
    if len(decoded) != 32:
        raise ContractError("source.checksum_sha256 must contain a SHA-256 digest")
