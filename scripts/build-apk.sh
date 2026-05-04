#!/usr/bin/env bash
set -euo pipefail

: "${VERSION:?VERSION required}"
: "${APP_NAME:?APP_NAME required}"
: "${ROOT_DIR:?ROOT_DIR required}"
: "${NDK_ROOT:?NDK_ROOT required}"
: "${NDK_TOOLCHAIN:?NDK_TOOLCHAIN required}"
: "${ANDROID_SYSROOT:?ANDROID_SYSROOT required}"
: "${BUILD_TOOLS:?BUILD_TOOLS required}"

cd "$ROOT_DIR"

python scripts/android-manifest.py "$VERSION" "$APP_NAME"
trap 'rm -f cmd/wt-gui/AndroidManifest.xml cmd/wt-gui/buildinfo_generated.go internal/appinfo/appinfo_generated.go' EXIT

if git describe --tags --exact-match HEAD >/dev/null 2>&1; then
	GIT_DATE=""
else
	GIT_DATE="$(git log -1 --format=%cd --date=format:%y%m%d%H%M%S 2>/dev/null || echo '')"
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

export ANDROID_HOME="$LOCALAPPDATA/Android/Sdk"
export ANDROID_NDK_HOME="$NDK_ROOT"
unset CGO_CFLAGS CGO_LDFLAGS CGO_LDFLAGS_ALLOW CC

fyne package --os android/arm64 \
	--app-id com.asolopovas.wtranscribe \
	--name "$APP_NAME" \
	--app-version "$VERSION" \
	--icon "$ROOT_DIR/winres/icon.png" \
	--src "$ROOT_DIR/cmd/wt-gui"

android_jar="$LOCALAPPDATA/Android/Sdk/platforms/android-36.1/android.jar"
if [ ! -f "$android_jar" ]; then
	android_jar="$(ls -d "$LOCALAPPDATA/Android/Sdk/platforms"/android-* 2>/dev/null | sort -V | tail -1)/android.jar"
fi

rm -rf dist/svc-build
mkdir -p dist/svc-build/classes
javac -source 1.8 -target 1.8 -Xlint:-options \
	-bootclasspath "$android_jar" -classpath "$android_jar" \
	-d dist/svc-build/classes \
	scripts/android-service/com/asolopovas/wtranscribe/WtForegroundService.java \
	scripts/android-service/com/asolopovas/wtranscribe/WtFileProvider.java

"$BUILD_TOOLS/d8.bat" --min-api 24 \
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

"$BUILD_TOOLS/zipalign.exe" -f 4 dist/unsigned.apk dist/aligned.apk

if [ ! -f "$USERPROFILE/.android/debug.keystore" ]; then
	keytool -genkey -v -keystore "$USERPROFILE/.android/debug.keystore" \
		-storepass android -alias androiddebugkey -keypass android \
		-keyalg RSA -keysize 2048 -validity 10000 \
		-dname "CN=Android Debug,O=Android,C=US"
fi

"$BUILD_TOOLS/apksigner.bat" sign \
	--ks "$USERPROFILE/.android/debug.keystore" \
	--ks-pass pass:android --key-pass pass:android \
	--ks-key-alias androiddebugkey \
	--out "dist/wt-${VERSION}.apk" dist/aligned.apk

rm -f dist/unsigned.apk dist/aligned.apk
rm -f "$apk" "${apk}.idsig"
echo "Built dist/wt-${VERSION}.apk"
