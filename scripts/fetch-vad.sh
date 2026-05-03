#!/usr/bin/env bash
set -euo pipefail

# Fetch the Silero VAD model so it can be embedded into the binary at build time.
# Idempotent: skips if file is already present and non-empty.

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
DEST_DIR="${ROOT_DIR}/internal/transcriber/assets"
FILE="ggml-silero-v6.2.0.bin"
URL="https://huggingface.co/ggml-org/whisper-vad/resolve/main/${FILE}"
DEST="${DEST_DIR}/${FILE}"

mkdir -p "${DEST_DIR}"

if [ -s "${DEST}" ]; then
	echo "VAD model already present: ${DEST} ($(stat -c%s "${DEST}" 2>/dev/null || stat -f%z "${DEST}") bytes)"
	exit 0
fi

echo "Fetching Silero VAD model -> ${DEST}"
if command -v curl >/dev/null 2>&1; then
	curl -fL --retry 3 -o "${DEST}.tmp" "${URL}"
elif command -v wget >/dev/null 2>&1; then
	wget -O "${DEST}.tmp" "${URL}"
else
	echo "fetch-vad.sh: need curl or wget" >&2
	exit 1
fi

mv "${DEST}.tmp" "${DEST}"
echo "Saved ${DEST} ($(stat -c%s "${DEST}" 2>/dev/null || stat -f%z "${DEST}") bytes)"
