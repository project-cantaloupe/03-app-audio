FROM python:3.12-slim AS runtime

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
