#!/usr/bin/env bash
# Upload artifacts to a GitHub release with retry. $1=tag, rest=files.
set -e
tag="$1"; shift
attempt=0
until gh release upload "$tag" "$@" --clobber; do
  attempt=$((attempt+1))
  if [ "$attempt" -ge 3 ]; then
    echo "ERROR: gh release upload failed after $attempt attempts" >&2
    exit 1
  fi
  echo "Upload attempt $attempt failed; retrying in 5s..." >&2
  sleep 5
done
