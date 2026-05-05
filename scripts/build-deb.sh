#!/usr/bin/env bash
# Build dist/wt_<VERSION>_amd64.deb. Args: $1=VERSION, $2=BINARY, $3=ROOT_DIR
set -e

VERSION="$1"
BINARY="$2"
ROOT_DIR="$3"

case "$VERSION" in
	dev-*) DEB_VERSION="0.0.0~$(echo "${VERSION#dev-}" | tr -d -)" ;;
	*)    DEB_VERSION="$VERSION" ;;
esac

if ! command -v dpkg-deb >/dev/null 2>&1; then
	echo "ERROR: dpkg-deb not found (install: sudo apt install dpkg-dev fakeroot)" >&2
	exit 1
fi

root="dist/wt-deb"
rm -rf "$root"
mkdir -p "$root/DEBIAN" "$root/opt/wt" "$root/opt/wt/models" \
	"$root/usr/bin" "$root/usr/share/applications" \
	"$root/usr/share/icons/hicolor/256x256/apps" \
	"$root/usr/share/doc/wt"

cp -P "dist/bin/$BINARY" "dist/bin/$BINARY-gui" "$root/opt/wt/"
cp -P dist/bin/*.so* "$root/opt/wt/" 2>/dev/null || true
cp dist/bin/diarize.py "$root/opt/wt/"
cp dist/deps/uv-linux "$root/opt/wt/uv"
cp scripts/setup-linux-user.sh "$root/opt/wt/wt-setup"
chmod +x "$root/opt/wt/uv" "$root/opt/wt/wt-setup" "$root/opt/wt/$BINARY" "$root/opt/wt/$BINARY-gui"

# Bundled models (sherpa-whisper-turbo ONNX)
src_models="${XDG_CONFIG_HOME:-$HOME/.config}/wt/models"
if [ -d "$src_models/sherpa-whisper-turbo" ]; then
	echo "  bundling sherpa-whisper-turbo/"
	mkdir -p "$root/opt/wt/models/sherpa-whisper-turbo"
	cp -r "$src_models/sherpa-whisper-turbo/." "$root/opt/wt/models/sherpa-whisper-turbo/"
else
	echo "  WARN: $src_models/sherpa-whisper-turbo missing (run 'task models-import' first)"
fi
# Docs
cp LICENSE "$root/usr/share/doc/wt/copyright" 2>/dev/null || true
cp THIRD-PARTY-LICENSES.txt "$root/usr/share/doc/wt/" 2>/dev/null || true
cp README.md "$root/usr/share/doc/wt/" 2>/dev/null || true
cp AGENTS.md "$root/usr/share/doc/wt/" 2>/dev/null || true

ln -sf "/opt/wt/$BINARY" "$root/usr/bin/$BINARY"
ln -sf "/opt/wt/$BINARY-gui" "$root/usr/bin/$BINARY-gui"
ln -sf /opt/wt/wt-setup "$root/usr/bin/wt-setup"

if [ -f winres/icon.png ]; then
	cp winres/icon.png "$root/usr/share/icons/hicolor/256x256/apps/wt.png"
fi

cat >"$root/usr/share/applications/wt.desktop" <<EOF
[Desktop Entry]
Type=Application
Name=wt
Comment=Whisper transcription with diarization
Exec=/opt/wt/wt-gui
Icon=wt
Terminal=false
Categories=AudioVideo;Audio;Utility;
EOF

inst_size=$(du -sk "$root/opt" | cut -f1)
cat >"$root/DEBIAN/control" <<EOF
Package: wt
Version: ${DEB_VERSION}
Section: sound
Priority: optional
Architecture: amd64
Maintainer: A. Solopovas <info@lyntouch.com>
Installed-Size: ${inst_size}
Depends: libc6, libgcc-s1, libstdc++6
Recommends: ffmpeg, nvidia-driver-535 | nvidia-driver-550 | nvidia-driver-560 | nvidia-driver-570
Description: Whisper-ONNX CLI + GUI with NeMo speaker diarization
 wt transcribes audio using sherpa-onnx (ONNX Runtime, CUDA-accelerated when
 an NVIDIA GPU is present) and performs speaker diarization via NVIDIA NeMo
 Sortformer. After install, run 'wt-setup' once to provision the per-user
 Python venv with NeMo (one-time, ~2 GB) and link bundled models into your
 config dir.
EOF

cat >"$root/DEBIAN/postinst" <<'EOF'
#!/bin/sh
set -e
if [ -d /usr/share/icons/hicolor ] && command -v gtk-update-icon-cache >/dev/null 2>&1; then
  gtk-update-icon-cache -f -t /usr/share/icons/hicolor 2>/dev/null || true
fi
if command -v update-desktop-database >/dev/null 2>&1; then
  update-desktop-database -q /usr/share/applications 2>/dev/null || true
fi
echo
echo "wt installed. To finish setup, each user must run once:"
echo "  wt-setup"
echo "(creates ~/.config/wt/python with nemo_toolkit[asr], ~2 GB download,"
echo " and links bundled ASR models into ~/.config/wt/models)"
EOF
chmod 0755 "$root/DEBIAN/postinst"

out="dist/wt_${VERSION}_amd64.deb"
fakeroot dpkg-deb --build -Zxz "$root" "$out"
ls -l "$out"
