#!/usr/bin/env bash
set -euo pipefail

ver="${1:?version required}"
binary="${2:?binary name required}"

dest="$LOCALAPPDATA/$binary"
if [ ! -d "$dest" ]; then
	echo "ERROR: $dest not found. Run installer first (dist/wt-setup-${ver}.exe)." >&2
	exit 1
fi

for proc in "$binary-gui.exe" "$binary.exe" llama-cli.exe; do
	taskkill //F //IM "$proc" 2>/dev/null || true
done

python -c "import time; time.sleep(2)"

copy_with_retry() {
	local src=$1 dst=$2
	for _ in 1 2 3 4 5 6 7 8 9 10; do
		if cp -f "$src" "$dst" 2>/dev/null; then return 0; fi
		sleep 1
	done
	cp -f "$src" "$dst"
}

for f in dist/bin/*.exe; do copy_with_retry "$f" "$dest/"; done
for f in dist/bin/*.dll; do [ -f "$f" ] && copy_with_retry "$f" "$dest/"; done
[ -f dist/bin/diarize.py ] && cp -f dist/bin/diarize.py "$dest/"

if [ -d dist/bin/llama ]; then
	mkdir -p "$dest/llama"
	for f in dist/bin/llama/*; do
		[ -f "$f" ] && copy_with_retry "$f" "$dest/llama/"
	done
fi

echo "Updated $dest"
"$dest/$binary.exe" --version
