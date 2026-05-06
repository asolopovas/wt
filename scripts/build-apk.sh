#!/usr/bin/env bash
set -euo pipefail

: "${VERSION:?VERSION required}"
: "${APP_NAME:?APP_NAME required}"
: "${ROOT_DIR:?ROOT_DIR required}"
: "${NDK_ROOT:?NDK_ROOT required}"
: "${NDK_TOOLCHAIN:?NDK_TOOLCHAIN required}"
: "${ANDROID_SYSROOT:?ANDROID_SYSROOT required}"
: "${BUILD_TOOLS:?BUILD_TOOLS required}"

ANDROID_SDK="${ANDROID_SDK:-${ANDROID_SDK_ROOT:-${ANDROID_HOME:-$HOME/Android/Sdk}}}"
USER_HOME="${USER_HOME:-$HOME}"
BIN_EXT="${BIN_EXT:-}"
EXE_EXT="${EXE_EXT:-}"

cd "$ROOT_DIR"

python scripts/android-manifest.py "$VERSION" "$APP_NAME"
trap 'rm -f cmd/wt-gui/AndroidManifest.xml cmd/wt-gui/buildinfo_generated.go internal/appinfo/appinfo_generated.go' EXIT

if git describe --tags --exact-match HEAD >/dev/null 2>&1; then
	GIT_DATE=""
else
	commit_ts="$(git log -1 --format=%cd --date=format:%Y-%m-%d-%H-%M-%S 2>/dev/null || true)"
	if [ -n "$commit_ts" ]; then
		GIT_DATE="dev-$commit_ts"
	else
		GIT_DATE=""
	fi
fi

cat >cmd/wt-gui/buildinfo_generated.go <<EOF
package main

func init() {
    Version = "$VERSION"
    BuildDate = "$GIT_DATE"
}
EOF

cat >internal/appinfo/appinfo_generated.go <<EOF
package appinfo

func init() {
    Name = "$APP_NAME"
    Version = "$VERSION"
    BuildDate = "$GIT_DATE"
}
EOF

export ANDROID_HOME="$ANDROID_SDK"
export ANDROID_NDK_HOME="$NDK_ROOT"
unset CGO_CFLAGS CGO_LDFLAGS CGO_LDFLAGS_ALLOW CC

if ! command -v fyne >/dev/null 2>&1; then
	if [ -x "$HOME/go/bin/fyne" ]; then
		export PATH="$HOME/go/bin:$PATH"
	else
		echo "ERROR: 'fyne' tool not found. Install with: go install fyne.io/fyne/v2/cmd/fyne@latest" >&2
		exit 1
	fi
fi

fyne package --os android/arm64 \
	--app-id com.asolopovas.wtranscribe \
	--name "$APP_NAME" \
	--icon "$ROOT_DIR/winres/icon.png" \
	--src "$ROOT_DIR/cmd/wt-gui"

android_jar=""
for cand in "$ANDROID_SDK/platforms/android-36.1/android.jar" \
	"$ANDROID_SDK/platforms/android-36/android.jar" \
	"$ANDROID_SDK/platforms/android-35/android.jar" \
	"$ANDROID_SDK/platforms/android-34/android.jar"; do
	if [ -f "$cand" ]; then android_jar="$cand"; break; fi
done
if [ -z "$android_jar" ]; then
	android_jar="$(ls -d "$ANDROID_SDK/platforms"/android-* 2>/dev/null | sort -V | tail -1)/android.jar"
fi
if [ ! -f "$android_jar" ]; then
	echo "ERROR: no android.jar found under $ANDROID_SDK/platforms/" >&2
	exit 1
fi

rm -rf dist/svc-build
mkdir -p dist/svc-build/classes
javac -source 1.8 -target 1.8 -Xlint:-options \
	-bootclasspath "$android_jar" -classpath "$android_jar" \
	-d dist/svc-build/classes \
	scripts/android-service/com/asolopovas/wtranscribe/WtForegroundService.java \
	scripts/android-service/com/asolopovas/wtranscribe/WtFileProvider.java

D8="$BUILD_TOOLS/d8${BIN_EXT}"
if [ ! -f "$D8" ]; then D8="$BUILD_TOOLS/d8"; fi
"$D8" --min-api 24 \
	--output dist/svc-build \
	dist/svc-build/classes/com/asolopovas/wtranscribe/WtForegroundService.class \
	dist/svc-build/classes/com/asolopovas/wtranscribe/WtFileProvider.class
test -f dist/svc-build/classes.dex

apk="cmd/wt-gui/$APP_NAME.apk"
mkdir -p dist
APK="$apk" \
	LIBCXX="$ANDROID_SYSROOT/usr/lib/aarch64-linux-android/libc++_shared.so" \
	LIBOMP="$NDK_TOOLCHAIN/lib/clang/18/lib/linux/aarch64/libomp.so" \
	SHERPA_BIN="dist/sherpa/sherpa-diar-android" \
	SHERPA_ASR_BIN="dist/sherpa/sherpa-asr-android" \
	SHERPA_SEG="dist/sherpa/models/sherpa-onnx-pyannote-segmentation-3-0/model.onnx" \
	SHERPA_EMB="dist/sherpa/models/titanet_large.onnx" \
	LLAMA_BIN="dist/llama/llama-cli-android-arm64" \
	FFMPEG_BIN="dist/ffmpeg/ffmpeg-android-arm64" \
	SVC_DEX="dist/svc-build/classes.dex" \
	OUT="dist/unsigned.apk" \
	python scripts/android-apk-patch.py

ZIPALIGN="$BUILD_TOOLS/zipalign${EXE_EXT}"
if [ ! -f "$ZIPALIGN" ]; then ZIPALIGN="$BUILD_TOOLS/zipalign"; fi
"$ZIPALIGN" -f 4 dist/unsigned.apk dist/aligned.apk

if [ ! -f "$USER_HOME/.android/debug.keystore" ]; then
	mkdir -p "$USER_HOME/.android"
	keytool -genkey -v -keystore "$USER_HOME/.android/debug.keystore" \
		-storepass android -alias androiddebugkey -keypass android \
		-keyalg RSA -keysize 2048 -validity 10000 \
		-dname "CN=Android Debug,O=Android,C=US"
fi

APKSIGNER="$BUILD_TOOLS/apksigner${BIN_EXT}"
if [ ! -f "$APKSIGNER" ]; then APKSIGNER="$BUILD_TOOLS/apksigner"; fi
"$APKSIGNER" sign \
	--ks "$USER_HOME/.android/debug.keystore" \
	--ks-pass pass:android --key-pass pass:android \
	--ks-key-alias androiddebugkey \
	--out "dist/wt-${VERSION}.apk" dist/aligned.apk

rm -f dist/unsigned.apk dist/aligned.apk
rm -f "$apk" "${apk}.idsig"
echo "Built dist/wt-${VERSION}.apk"
