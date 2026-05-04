#!/usr/bin/env bash
set -euo pipefail

ver="${1:?version required}"
binary="${2:?binary name required}"
mode="${3:-full}"

dest=/opt/wt

stop_running() {
	pkill -x "$binary-gui" 2>/dev/null || true
	pkill -x "$binary" 2>/dev/null || true
	sleep 0.3
}

if [ "$mode" = "quick" ]; then
	if [ ! -d "$dest" ]; then
		echo "ERROR: $dest not found. Run 'task install' once (without QUICK=1) to install the .deb first." >&2
		exit 1
	fi
	stop_running
	sudo install -m 0755 "dist/bin/$binary" "$dest/$binary"
	sudo install -m 0755 "dist/bin/$binary-gui" "$dest/$binary-gui"
	for f in dist/bin/lib*.so*; do
		[ -e "$f" ] && sudo cp -Pf "$f" "$dest/"
	done
	"$binary" --version
	exit 0
fi

deb="dist/wt_${ver}_amd64.deb"
if [ ! -f "$deb" ]; then
	echo "ERROR: $deb not found" >&2
	exit 1
fi
stop_running
sudo dpkg -i "$deb"
"$binary" --version
