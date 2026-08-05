import json
from pathlib import Path

import pytest

from audio_worker.contracts import ContractError, TranscodeJob


def example_job():
    path = Path(__file__).parents[3] / "shared" / "schema" / "examples" / "transcode-job.json"
    return json.loads(path.read_text(encoding="utf-8"))


def test_parse_contract_example():
    job = TranscodeJob.from_dict(example_job())

    assert job.schema_version == 1
    assert job.targets.mp3_bitrate_kbps == 192
    assert job.source.bucket == "cntlp-aws-quarantine"


def test_reject_unknown_queue_field():
    value = example_job()
    value["presigned_url"] = "https://must-not-enter-queue.example"

    with pytest.raises(ContractError, match="unknown"):
        TranscodeJob.from_dict(value)


def test_reject_invalid_checksum():
    value = example_job()
    value["source"]["checksum_sha256"] = "not-a-checksum"

    with pytest.raises(ContractError, match="base64"):
        TranscodeJob.from_dict(value)
