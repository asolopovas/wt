#!/usr/bin/env bash
# Build a static, audio-only ffmpeg for android-arm64 using the project NDK.
# Output: dist/ffmpeg/ffmpeg-android-arm64 (single ELF, ~3-4 MB).

set -euo pipefail

VERSION="${FFMPEG_VERSION:-7.1.1}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
NDK="${ANDROID_NDK_HOME:-${LOCALAPPDATA:-}/Android/Sdk/ndk/27.2.12479018}"
NDK="${NDK//\\//}"
TOOLCHAIN="$NDK/toolchains/llvm/prebuilt/windows-x86_64"
if [ ! -d "$TOOLCHAIN" ]; then
    for host in linux-x86_64 darwin-x86_64 darwin-arm64; do
        if [ -d "$NDK/toolchains/llvm/prebuilt/$host" ]; then
            TOOLCHAIN="$NDK/toolchains/llvm/prebuilt/$host"
            break
        fi
    done
fi

SYSROOT="$TOOLCHAIN/sysroot"
API=28
ARCH=aarch64

OUT_DIR="$ROOT/dist/ffmpeg"
OUT="$OUT_DIR/ffmpeg-android-arm64"
SRC_ROOT="$OUT_DIR/src"
BUILD_DIR="$OUT_DIR/build-arm64"

if [ -f "$OUT" ] && [ -f "$OUT.version" ] && [ "$(cat "$OUT.version" 2>/dev/null)" = "$VERSION" ]; then
    echo "ffmpeg-android-arm64 cached: $OUT (v$VERSION)"
    exit 0
fi

if [ ! -d "$TOOLCHAIN" ]; then
    echo "ERROR: NDK toolchain not found at $TOOLCHAIN" >&2
    exit 1
fi

mkdir -p "$SRC_ROOT" "$BUILD_DIR" "$OUT_DIR"

TARBALL="$SRC_ROOT/ffmpeg-$VERSION.tar.xz"
SRC_DIR="$SRC_ROOT/ffmpeg-$VERSION"
if [ ! -f "$TARBALL" ]; then
    echo "Downloading FFmpeg $VERSION..."
    curl -fsSL "https://ffmpeg.org/releases/ffmpeg-$VERSION.tar.xz" -o "$TARBALL"
fi
if [ ! -d "$SRC_DIR" ]; then
    echo "Extracting..."
    tar -xf "$TARBALL" -C "$SRC_ROOT"
fi

CC="$TOOLCHAIN/bin/aarch64-linux-android${API}-clang"
CXX="$TOOLCHAIN/bin/aarch64-linux-android${API}-clang++"
AR="$TOOLCHAIN/bin/llvm-ar"
RANLIB="$TOOLCHAIN/bin/llvm-ranlib"
STRIP="$TOOLCHAIN/bin/llvm-strip"
NM="$TOOLCHAIN/bin/llvm-nm"

# clang.exe / clang vs clang on host
for c in "$CC.cmd" "$CC.exe" "$CC"; do
    if [ -f "$c" ]; then CC="$c"; break; fi
done
for c in "$CXX.cmd" "$CXX.exe" "$CXX"; do
    if [ -f "$c" ]; then CXX="$c"; break; fi
done
for tool in AR RANLIB STRIP NM; do
    val="${!tool}"
    for c in "$val.exe" "$val"; do
        if [ -f "$c" ]; then printf -v "$tool" '%s' "$c"; break; fi
    done
done

cd "$BUILD_DIR"

if [ ! -f config.h ] || [ "${FFMPEG_RECONFIGURE:-0}" = "1" ]; then
    echo "Configuring..."
    "$SRC_DIR/configure" \
        --prefix="$BUILD_DIR/install" \
        --target-os=android \
        --arch="$ARCH" \
        --cpu=armv8-a \
        --enable-cross-compile \
        --cross-prefix="$TOOLCHAIN/bin/llvm-" \
        --cc="$CC" \
        --cxx="$CXX" \
        --ar="$AR" \
        --ranlib="$RANLIB" \
        --strip="$STRIP" \
        --nm="$NM" \
        --sysroot="$SYSROOT" \
        --enable-static --disable-shared \
        --enable-pic \
        --disable-everything \
        --disable-doc --disable-htmlpages --disable-manpages --disable-podpages --disable-txtpages \
        --disable-debug \
        --disable-network \
        --enable-protocol=file,pipe \
        --enable-filter=aresample,aformat,anull,atrim,volume,silenceremove,copy,trim,concat \
        --enable-demuxer=mov,matroska,ogg,wav,flac,mp3,aac,m4v,asf,aiff,w64,caf \
        --enable-muxer=mov,mp4,ipod,wav,flac,adts,ogg,matroska,pcm_s16le,pcm_s16be,pcm_s24le,pcm_f32le \
        --enable-decoder=aac,aac_latm,mp3,mp3float,flac,vorbis,opus,pcm_s16le,pcm_s16be,pcm_s24le,pcm_s32le,pcm_f32le,pcm_u8,alac,mp2,mp2float,mp1,mp1float,wmav1,wmav2 \
        --enable-encoder=aac,flac,pcm_s16le,pcm_s24le,pcm_f32le \
        --enable-parser=aac,flac,opus,vorbis,mpegaudio,ac3 \
        --enable-bsf=aac_adtstoasc \
        --disable-symver \
        --disable-asm \
        --extra-cflags="-Os -fPIC -DANDROID -ffunction-sections -fdata-sections" \
        --extra-ldflags="-Wl,-z,max-page-size=16384 -Wl,--gc-sections"
fi

JOBS="${JOBS:-${NUMBER_OF_PROCESSORS:-4}}"
MAKE_BIN="${MAKE_BIN:-}"
if [ -z "$MAKE_BIN" ]; then
    for m in make mingw32-make gmake; do
        if command -v "$m" >/dev/null 2>&1; then MAKE_BIN="$m"; break; fi
    done
fi
if [ -z "$MAKE_BIN" ]; then
    echo "ERROR: no make/mingw32-make found in PATH" >&2
    exit 1
fi
echo "Building with $MAKE_BIN -j$JOBS..."
"$MAKE_BIN" -j"$JOBS"

cp ffmpeg "$OUT"
"$STRIP" "$OUT" 2>/dev/null || true
printf '%s' "$VERSION" > "$OUT.version"
size_kb=$(du -k "$OUT" | cut -f1)
echo "Built: $OUT (${size_kb} KB, FFmpeg $VERSION)"
