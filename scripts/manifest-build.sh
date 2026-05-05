#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SRC="$ROOT/internal/models/manifest.source.json"
OUT="$ROOT/internal/models/manifest.json"

if [ ! -f "$SRC" ]; then
	echo "ERROR: $SRC missing." >&2
	exit 1
fi

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

probe_one() {
	local idx="$1" url="$2" outf="$3"
	local headers size etag
	headers="$(curl --connect-timeout 10 --max-time 30 -sIL "$url" 2>/dev/null | tr -d '\r' || true)"
	size="$(printf '%s\n' "$headers" | awk -F': ' 'tolower($1)=="x-linked-size"{v=$2} END{print v+0}')"
	etag="$(printf '%s\n' "$headers" | awk -F': ' 'tolower($1)=="x-linked-etag"{v=$2} END{gsub(/"/,"",v); print v}')"
	if [ -z "$size" ] || [ "$size" -eq 0 ]; then
		size="$(printf '%s\n' "$headers" | awk -F': ' 'tolower($1)=="content-length"{v=$2} END{print v+0}')"
	fi
	if [ -z "$etag" ] || [ "${#etag}" -ne 64 ]; then
		local tmp="$work/blob.$idx"
		curl --connect-timeout 10 --max-time 120 -sL "$url" -o "$tmp"
		etag="$(sha256sum "$tmp" | awk '{print $1}')"
		size="$(stat -c%s "$tmp")"
		rm -f "$tmp"
	fi
	printf '%s\t%s\t%s\n' "$idx" "$size" "$etag" > "$outf"
}

mapfile -t entries < <(jq -c '.entries[]' "$SRC")

idx=0
for entry in "${entries[@]}"; do
	id="$(jq -r '.id' <<<"$entry")"
	count="$(jq '.files | length' <<<"$entry")"
	echo "[$id] $count file(s)" >&2
	for i in $(seq 0 $((count - 1))); do
		url="$(jq -r ".files[$i].url" <<<"$entry")"
		probe_one "$idx" "$url" "$work/r.$idx" &
		printf '%s\t%s\t%s\t%s\n' "$idx" "$id" "$i" "$url" >> "$work/index"
		idx=$((idx + 1))
	done
done

wait

while IFS=$'\t' read -r ix id i url; do
	if [ ! -f "$work/r.$ix" ]; then
		echo "FAIL  $id  file[$i]  $url (no result)" >&2
		exit 1
	fi
	IFS=$'\t' read -r _ size etag < "$work/r.$ix"
	if [ -z "$size" ] || [ "$size" -eq 0 ] || [ "${#etag}" -ne 64 ]; then
		echo "FAIL  $id  file[$i]  $url  (size=$size etag=$etag)" >&2
		exit 1
	fi
	echo "  ok  $id  file[$i]  size=$size sha256=${etag:0:12}…  $url" >&2
	tmp="$work/manifest.$ix"
	jq --arg id "$id" --argjson i "$i" --arg sz "$size" --arg h "$etag" \
		'(.entries[] | select(.id == $id) | .files[$i].sizeBytes) |= ($sz | tonumber)
		 | (.entries[] | select(.id == $id) | .files[$i].sha256) |= $h' \
		"$SRC" >"$tmp"
	mv "$tmp" "$SRC"
done < "$work/index"

jq '.entries[] |= (.sizeBytes = (.files | map(.sizeBytes) | add))' "$SRC" >"$work/final"
mv "$work/final" "$SRC"

cp "$SRC" "$OUT"
echo "Wrote $OUT" >&2
echo "Total entries: $(jq '.entries | length' "$OUT")" >&2
echo "Total bytes:   $(jq '[.entries[].sizeBytes] | add' "$OUT")" >&2
