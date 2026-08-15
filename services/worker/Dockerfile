# `python:3.12-slim` resolves to Debian bookworm, whose ffmpeg is the 5.1 line.
# That line carried 560 findings here and -- the part that matters -- only 59 of
# them were fixable: the nine libav* packages each had 13 High/Critical CVEs
# with no patched version in bookworm at all. No amount of `apt-get upgrade`
# reaches a fix that the distribution never published.
#
# trixie (Debian 13) ships the actively maintained ffmpeg 7.x instead. Moving
# the distribution is the only remediation available, because the worker runs
# the ffmpeg/ffprobe binaries directly (services/worker/src/audio_worker/media.py)
# and cannot drop the dependency.
#
# The tag is pinned to the distribution explicitly. `-slim` alone would have
# floated to trixie on its own schedule, which is the same change arriving
# unannounced on some unrelated build.
FROM python:3.13-slim-trixie AS runtime

RUN apt-get update \
    && apt-get install --no-install-recommends -y ffmpeg \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --system --gid 10001 cntlp \
    && useradd --system --uid 10001 --gid cntlp --home-dir /app cntlp

WORKDIR /app

COPY pyproject.toml ./
COPY src ./src

RUN pip install --no-cache-dir .

USER 10001:10001

EXPOSE 9090

ENTRYPOINT ["audio-worker"]
