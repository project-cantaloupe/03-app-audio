#!/usr/bin/env bash
set -euo pipefail

for command in curl docker jq openssl; do
  command -v "$command" >/dev/null || {
    echo "required command not found: $command" >&2
    exit 1
  }
done

repository_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
workdir=$(mktemp -d "${TMPDIR:-/tmp}/cntlp-audio-e2e.XXXXXX")
trap 'rm -rf "$workdir"' EXIT

api_url=${AUDIO_API_URL:-http://localhost:8080}
owner_subject=${AUDIO_OWNER_SUBJECT:-local-e2e-user}
source_file="$workdir/source.wav"

(cd "$repository_root" && docker compose exec -T audio-transcode \
  ffmpeg -nostdin -hide_banner -loglevel error \
  -f lavfi -i "sine=frequency=440:duration=2" \
  -ac 2 -ar 44100 -c:a pcm_s16le -f wav pipe:1) >"$source_file"

if file_size=$(stat -f %z "$source_file" 2>/dev/null); then
  :
else
  file_size=$(stat -c %s "$source_file")
fi
checksum=$(openssl dgst -sha256 -binary "$source_file" | openssl base64 -A)

create_payload=$(jq -n \
  --arg title "Cantaloupe local E2E" \
  --arg content_type "audio/wav" \
  --arg checksum "$checksum" \
  --argjson content_length "$file_size" \
  '{title:$title,content_type:$content_type,content_length:$content_length,checksum_sha256:$checksum}')

create_response=$(curl --fail-with-body --silent --show-error \
  --header "Content-Type: application/json" \
  --header "X-Cantaloupe-Subject: $owner_subject" \
  --data "$create_payload" \
  "$api_url/v1/audios/uploads")

audio_id=$(jq -er '.audio_id' <<<"$create_response")
upload_url=$(jq -er '.upload_url' <<<"$create_response")

curl --fail-with-body --silent --show-error \
  --request PUT \
  --header "Content-Type: audio/wav" \
  --header "x-amz-checksum-sha256: $checksum" \
  --data-binary "@$source_file" \
  "$upload_url" >/dev/null

curl --fail-with-body --silent --show-error \
  --request POST \
  --header "X-Cantaloupe-Subject: $owner_subject" \
  "$api_url/v1/audios/$audio_id/complete" >/dev/null

status=""
for _ in $(seq 1 90); do
  record=$(curl --fail-with-body --silent --show-error \
    --header "X-Cantaloupe-Subject: $owner_subject" \
    "$api_url/v1/audios/$audio_id")
  status=$(jq -r '.status' <<<"$record")
  case "$status" in
    READY) break ;;
    QUARANTINED|SCAN_FAILED|TRANSCODE_FAILED)
      echo "audio processing failed with status: $status" >&2
      exit 1
      ;;
  esac
  sleep 1
done

if [[ "$status" != "READY" ]]; then
  echo "audio did not become READY before timeout; last status: $status" >&2
  exit 1
fi

playback_response=$(curl --fail-with-body --silent --show-error \
  --header "X-Cantaloupe-Subject: $owner_subject" \
  "$api_url/v1/audios/$audio_id/playback")
playback_url=$(jq -er '.playback_url' <<<"$playback_response")
waveform_url=$(jq -er '.waveform_url' <<<"$playback_response")

curl --fail-with-body --silent --show-error --range 0-31 \
  "$playback_url" --output "$workdir/playback.sample"
curl --fail-with-body --silent --show-error \
  "$waveform_url" --output "$workdir/waveform.json"
jq -e '.schema_version == 1 and .duration_ms > 0 and (.peaks | length > 0)' \
  "$workdir/waveform.json" >/dev/null

jq -n \
  --arg audio_id "$audio_id" \
  --arg status "$status" \
  --arg expires_at "$(jq -r '.expires_at' <<<"$playback_response")" \
  --argjson waveform_points "$(jq '.peaks | length' "$workdir/waveform.json")" \
  '{audio_id:$audio_id,status:$status,playback_access:"verified",waveform_points:$waveform_points,expires_at:$expires_at}'
