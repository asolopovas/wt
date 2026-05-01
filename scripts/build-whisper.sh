#!/usr/bin/env bash
# Build whisper.cpp for the host platform.
# Args: $1 = WHISPER_ROOT, $2 = WHISPER_BUILD, $3 = CUDA_DIR (may be empty),
#       $4 = CUDA_ARCH, $5 = ROOT_DIR, $6 = LINUX_CUDA flag, $7 = IS_LINUX flag
#
# IS_LINUX is passed from Taskfile rather than re-detected via `uname -s`.
# When `bash scripts/...` resolves to WSL bash on a Windows host (because
# bash.exe is on PATH ahead of MSYS), uname -s reports Linux and the script
# wrongly took the Linux branch, attempting to cd into an unbuilt source tree.
set -e

WHISPER_ROOT="$1"
WHISPER_BUILD="$2"
CUDA_DIR="$3"
CUDA_ARCH="$4"
ROOT_DIR="$5"
LINUX_CUDA="$6"
IS_LINUX="$7"

jobs() { echo "${NUMBER_OF_PROCESSORS:-$(nproc 2>/dev/null || echo 4)}"; }

if [ "$IS_LINUX" = "1" ]; then
  [ -f "$WHISPER_BUILD/src/libwhisper.so" ] && exit 0
  echo "Linux: building whisper.cpp (CUDA=$LINUX_CUDA)..."
  cd "$WHISPER_ROOT"
  extra=""
  if [ -n "$LINUX_CUDA" ]; then
    extra="-DGGML_CUDA=ON -DCMAKE_CUDA_ARCHITECTURES=native"
  fi
  cmake -B build \
    -DCMAKE_BUILD_TYPE=Release \
    -DBUILD_SHARED_LIBS=ON \
    -DCMAKE_POSITION_INDEPENDENT_CODE=ON \
    -DWHISPER_BUILD_EXAMPLES=OFF \
    -DWHISPER_BUILD_TESTS=OFF \
    $extra
  cmake --build build -j"$(jobs)"
  exit 0
fi

if [ -n "$CUDA_DIR" ]; then
  [ -f "$WHISPER_BUILD/bin/whisper.dll" ] && exit 0
  echo "CUDA detected: $CUDA_DIR (arch $CUDA_ARCH)"
  echo "Building whisper.cpp with CUDA (via MSVC + Ninja)..."
  export CUDA_ARCH
  cmd.exe /c "${ROOT_DIR}/scripts/build-whisper-cuda.bat" "$WHISPER_ROOT"
  exit 0
fi

[ -f "$WHISPER_ROOT/build/src/libwhisper.a" ] && exit 0
echo "CUDA not found, building CPU-only..."
cd "$WHISPER_ROOT"
cmake -B build -G "MinGW Makefiles" \
  -DCMAKE_BUILD_TYPE=Release \
  -DBUILD_SHARED_LIBS=OFF \
  -DWHISPER_BUILD_EXAMPLES=OFF \
  -DWHISPER_BUILD_TESTS=OFF
cmake --build build -j"$(jobs)"
cd build/ggml/src
for f in ggml.a ggml-base.a ggml-cpu.a; do
  [ -f "$f" ] && cp "$f" "lib$f" 2>/dev/null
done
