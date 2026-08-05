from __future__ import annotations

import array
import json
import subprocess
import sys
from collections.abc import Iterable, Sequence
from dataclasses import dataclass
from pathlib import Path
from typing import Any


class MediaError(RuntimeError):
    pass


@dataclass(frozen=True)
class MediaInfo:
    duration_ms: int
    codec_name: str


def probe(path: Path, maximum_duration_seconds: int) -> MediaInfo:
    command = [
        "ffprobe",
        "-v",
        "error",
        "-select_streams",
        "a:0",
        "-show_entries",
        "stream=codec_name:format=duration",
        "-of",
        "json",
        str(path),
    ]
    completed = subprocess.run(command, check=False, capture_output=True, text=True)
    if completed.returncode != 0:
        raise MediaError("ffprobe rejected the uploaded media")
    try:
        payload: dict[str, Any] = json.loads(completed.stdout)
        streams = payload["streams"]
        duration_seconds = float(payload["format"]["duration"])
        codec_name = str(streams[0]["codec_name"])
    except (KeyError, IndexError, TypeError, ValueError, json.JSONDecodeError) as error:
        raise MediaError("ffprobe did not return a valid audio stream") from error
    if duration_seconds <= 0 or duration_seconds > maximum_duration_seconds:
        raise MediaError("audio duration is outside the allowed range")
    return MediaInfo(duration_ms=round(duration_seconds * 1000), codec_name=codec_name)


def transcode_mp3(source: Path, destination: Path, bitrate_kbps: int) -> None:
    command = [
        "ffmpeg",
        "-nostdin",
        "-hide_banner",
        "-loglevel",
        "error",
        "-y",
        "-i",
        str(source),
        "-map",
        "0:a:0",
        "-vn",
        "-map_metadata",
        "-1",
        "-ac",
        "2",
        "-codec:a",
        "libmp3lame",
        "-b:a",
        f"{bitrate_kbps}k",
        str(destination),
    ]
    completed = subprocess.run(command, check=False, capture_output=True, text=True)
    if completed.returncode != 0:
        raise MediaError("ffmpeg could not create the MP3 artifact")


def write_waveform(
    source: Path,
    destination: Path,
    duration_ms: int,
    points_per_second: int,
    sample_rate: int = 8000,
) -> None:
    command = [
        "ffmpeg",
        "-nostdin",
        "-hide_banner",
        "-loglevel",
        "error",
        "-i",
        str(source),
        "-map",
        "0:a:0",
        "-vn",
        "-ac",
        "1",
        "-ar",
        str(sample_rate),
        "-f",
        "s16le",
        "pipe:1",
    ]
    process = subprocess.Popen(command, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
    if process.stdout is None:
        raise MediaError("ffmpeg waveform output was not available")

    samples_per_point = max(1, sample_rate // points_per_second)
    peaks: list[list[int]] = []
    pending = bytearray()
    bytes_per_point = samples_per_point * 2
    while True:
        chunk = process.stdout.read(64 * 1024)
        if not chunk:
            break
        pending.extend(chunk)
        while len(pending) >= bytes_per_point:
            window = bytes(pending[:bytes_per_point])
            del pending[:bytes_per_point]
            peaks.append(_peak_pair(_pcm16_samples(window)))
    if pending:
        if len(pending) % 2:
            pending.pop()
        if pending:
            peaks.append(_peak_pair(_pcm16_samples(bytes(pending))))

    stderr = process.stderr.read() if process.stderr is not None else b""
    if process.wait() != 0:
        detail = stderr.decode(errors="replace")[:200]
        raise MediaError(f"ffmpeg could not create waveform data: {detail}")

    payload = {
        "schema_version": 1,
        "duration_ms": duration_ms,
        "points_per_second": points_per_second,
        "bits": 8,
        "channels": 1,
        "peaks": peaks,
    }
    destination.write_text(json.dumps(payload, separators=(",", ":")), encoding="utf-8")


def calculate_peaks(samples: Sequence[int], samples_per_point: int) -> list[list[int]]:
    if samples_per_point <= 0:
        raise ValueError("samples_per_point must be positive")
    return [
        _peak_pair(samples[index : index + samples_per_point])
        for index in range(0, len(samples), samples_per_point)
    ]


def _pcm16_samples(value: bytes) -> Iterable[int]:
    samples = array.array("h")
    samples.frombytes(value)
    if sys.byteorder != "little":
        samples.byteswap()
    return samples


def _peak_pair(samples: Iterable[int]) -> list[int]:
    values = list(samples)
    if not values:
        return [0, 0]
    low = max(-128, min(127, round(min(values) * 128 / 32768)))
    high = max(-128, min(127, round(max(values) * 127 / 32767)))
    return [low, high]
