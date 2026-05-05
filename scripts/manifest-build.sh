#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SRC="$ROOT/internal/default_config.yml"

if [ ! -f "$SRC" ]; then
	echo "ERROR: $SRC missing." >&2
	exit 1
fi

cache="$HOME/.cache/wt-manifest-build"
mkdir -p "$cache"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

python3 - "$SRC" >"$work/index" <<'PY'
import sys, yaml
data = yaml.safe_load(open(sys.argv[1]))
for ei, e in enumerate(data.get("models", [])):
    for fi, f in enumerate(e.get("files", [])):
        print(f"{ei}|{fi}|{e['id']}|{f['url']}")
PY

count=$(wc -l <"$work/index")
echo "$count files to download/verify (parallel=4, cache=$cache)" >&2

probe() {
	local rec="$1"
	local ei fi id url
	ei="$(printf '%s' "$rec" | cut -d'|' -f1)"
	fi="$(printf '%s' "$rec" | cut -d'|' -f2)"
	id="$(printf '%s' "$rec" | cut -d'|' -f3)"
	url="$(printf '%s' "$rec" | cut -d'|' -f4-)"

	local key hash_file tmp
	key="$(printf '%s' "$url" | sha256sum | cut -c1-32)"
	hash_file="$cache/$key.bin"

	if [ ! -s "$hash_file" ]; then
		tmp="$hash_file.partial"
		if ! curl -fsSL --connect-timeout 30 --max-time 1200 --retry 3 --retry-delay 5 \
			-o "$tmp" "$url"; then
			echo "FAIL  [$id] file[$fi]  $url" >&2
			return 1
		fi
		mv "$tmp" "$hash_file"
	fi

	local sz sh
	sz="$(stat -c%s "$hash_file")"
	sh="$(sha256sum "$hash_file" | awk '{print $1}')"
	echo "  ok  [$id] file[$fi]  size=$sz  sha=${sh:0:12}…" >&2
	printf '%s|%s|%s|%s\n' "$ei" "$fi" "$sz" "$sh" >>"$work/results"
}

export -f probe
export cache work

: >"$work/results"
running=0
maxpar=4
fails=0
while IFS= read -r rec; do
	probe "$rec" || fails=$((fails + 1)) &
	running=$((running + 1))
	if [ "$running" -ge "$maxpar" ]; then
		wait -n
		running=$((running - 1))
	fi
done <"$work/index"
wait

if [ "$fails" -gt 0 ]; then
	echo "ERROR: $fails downloads failed" >&2
	exit 1
fi

python3 - "$SRC" "$work/results" >"$work/out.yml" <<'PY'
import sys, yaml
data = yaml.safe_load(open(sys.argv[1]))
results = {}
for line in open(sys.argv[2]):
    parts = line.rstrip("\n").split("|")
    if len(parts) != 4:
        continue
    ei, fi, sz, sh = int(parts[0]), int(parts[1]), int(parts[2]), parts[3]
    results[(ei, fi)] = (sz, sh)

for ei, e in enumerate(data.get("models", [])):
    total = 0
    for fi, f in enumerate(e.get("files", [])):
        sz, sh = results[(ei, fi)]
        f["sizeBytes"] = sz
        f["sha256"] = sh
        total += sz
    e["sizeBytes"] = total

class Dumper(yaml.SafeDumper):
    pass
Dumper.add_representer(str, lambda d, v: d.represent_scalar("tag:yaml.org,2002:str", v, style='"' if any(c in v for c in ":,#") else None))

print(yaml.dump(data, Dumper=Dumper, sort_keys=False, default_flow_style=False, width=200, allow_unicode=True))
PY

mv "$work/out.yml" "$SRC"
echo "Wrote $SRC" >&2
echo "Cache: $cache ($(du -sh "$cache" | cut -f1))" >&2
