#!/usr/bin/env bash
set -euo pipefail

SHERPA_VER="v1.13.0"
SHERPA_CPU_URL="https://github.com/k2-fsa/sherpa-onnx/releases/download/${SHERPA_VER}/sherpa-onnx-${SHERPA_VER}-linux-x64-shared-no-tts.tar.bz2"
SHERPA_CUDA_URL="https://github.com/k2-fsa/sherpa-onnx/releases/download/${SHERPA_VER}/sherpa-onnx-${SHERPA_VER}-cuda-12.x-cudnn-9.x-linux-x64-gpu.tar.bz2"
LLAMA_VER="b9041"
LLAMA_URL="https://github.com/ggml-org/llama.cpp/releases/download/${LLAMA_VER}/llama-${LLAMA_VER}-bin-ubuntu-x64.zip"

UV="/opt/wt/uv"
BUNDLED_MODELS="/opt/wt/models"
cfgdir="${XDG_CONFIG_HOME:-$HOME/.config}/wt"
modelsdir="$cfgdir/models"
pydir="$cfgdir/python"
sherpa_cpu_dir="$cfgdir/sherpa-cpu"
sherpa_cuda_dir="$cfgdir/sherpa-cuda"
llama_dir="$cfgdir/llama"
mkdir -p "$cfgdir" "$modelsdir" "$llama_dir"

fetch_tarball() {
  local url="$1" tmp
  tmp="$(mktemp -d)"
  echo "      → $url"
  curl -fSL --retry 3 -o "$tmp/archive" "$url"
  case "$url" in
    *.tar.bz2) tar -xjf "$tmp/archive" -C "$tmp" ;;
    *.tar.gz)  tar -xzf "$tmp/archive" -C "$tmp" ;;
    *.zip)     unzip -q -o "$tmp/archive" -d "$tmp/extracted" ;;
  esac
  echo "$tmp"
}

first_subdir() {
  find "$1" -maxdepth 1 -mindepth 1 -type d | head -1
}

ensure_sherpa_cpu() {
  if [ -x "$sherpa_cpu_dir/bin/sherpa-onnx-offline" ]; then
    echo "      sherpa-onnx CPU runtime already present"
    return 0
  fi
  echo "      installing sherpa-onnx CPU runtime"
  local tmp; tmp=$(fetch_tarball "$SHERPA_CPU_URL")
  local inner; inner=$(first_subdir "$tmp")
  rm -rf "$sherpa_cpu_dir"
  mv "$inner" "$sherpa_cpu_dir"
  chmod +x "$sherpa_cpu_dir/bin/"sherpa-onnx-* 2>/dev/null || true
  rm -rf "$tmp"
}

ensure_sherpa_cuda() {
  command -v nvidia-smi >/dev/null 2>&1 || return 0
  if [ -x "$sherpa_cuda_dir/bin/sherpa-onnx-offline" ]; then
    echo "      sherpa-onnx CUDA runtime already present"
    return 0
  fi
  echo "      installing sherpa-onnx CUDA runtime"
  local tmp; tmp=$(fetch_tarball "$SHERPA_CUDA_URL")
  local inner; inner=$(first_subdir "$tmp")
  rm -rf "$sherpa_cuda_dir"
  mv "$inner" "$sherpa_cuda_dir"
  chmod +x "$sherpa_cuda_dir/bin/"sherpa-onnx-* 2>/dev/null || true
  rm -rf "$tmp"
}

ensure_llama() {
  if [ -x "$llama_dir/llama-cli" ]; then
    echo "      llama.cpp already present"
    return 0
  fi
  echo "      installing llama.cpp ${LLAMA_VER}"
  local tmp; tmp=$(fetch_tarball "$LLAMA_URL")
  local src
  if [ -f "$tmp/extracted/llama-cli" ]; then
    src="$tmp/extracted"
  elif [ -f "$tmp/extracted/build/bin/llama-cli" ]; then
    src="$tmp/extracted/build/bin"
  else
    src=$(dirname "$(find "$tmp/extracted" -maxdepth 4 -type f -name llama-cli | head -1)")
  fi
  cp -f "$src/llama-cli" "$llama_dir/"
  cp -P "$src/"*.so* "$llama_dir/" 2>/dev/null || true
  chmod +x "$llama_dir/llama-cli"
  rm -rf "$tmp"
}

link_cuda_runtime_for_sherpa() {
  [ -d "$sherpa_cuda_dir/lib" ] || return 0
  local nv_root="$pydir/lib/python3.12/site-packages/nvidia"
  [ -d "$nv_root" ] || return 0
  echo "      linking CUDA runtime libs from torch venv into sherpa-cuda/lib"
  for sub in cudnn cublas cuda_runtime cuda_nvrtc; do
    [ -d "$nv_root/$sub/lib" ] || continue
    cp -P "$nv_root/$sub/lib/"lib*.so* "$sherpa_cuda_dir/lib/" 2>/dev/null || true
  done
}

echo "[1/5] Installing sherpa-onnx CPU runtime"
ensure_sherpa_cpu
echo "[2/5] Installing sherpa-onnx CUDA runtime (if NVIDIA GPU present)"
ensure_sherpa_cuda
echo "[3/5] Installing llama.cpp runtime"
ensure_llama
echo "[4/5] Linking bundled models -> $modelsdir"
if [ -d "$BUNDLED_MODELS" ]; then
  for f in "$BUNDLED_MODELS"/*.bin; do
    [ -f "$f" ] || continue
    name="$(basename "$f")"
    if [ -e "$modelsdir/$name" ]; then
      echo "  skip $name (already present)"
    else
      ln -s "$f" "$modelsdir/$name"
      echo "  linked $name"
    fi
  done
else
  echo "  no bundled models at $BUNDLED_MODELS"
fi

if [ ! -x "$UV" ]; then
  echo "ERROR: $UV not found (is the wt package installed?)"; exit 1
fi

echo "[5/5] Creating Python 3.12 venv at $pydir"
if [ ! -x "$pydir/bin/python" ]; then
  "$UV" venv "$pydir" --python 3.12 --python-preference only-managed
fi
VPY="$pydir/bin/python"

echo "      Installing nemo_toolkit[asr] (~2 GB)"
if ! "$VPY" -c "import nemo" 2>/dev/null; then
  "$UV" pip install "nemo_toolkit[asr]" --python "$VPY"
else
  echo "      already installed"
fi

echo "      CUDA torch (optional)"
if command -v nvidia-smi >/dev/null 2>&1; then
  echo "      NVIDIA GPU detected; installing torch with CUDA..."
  "$UV" pip install torch torchvision torchaudio \
    --index-url https://download.pytorch.org/whl/cu124 --python "$VPY" || \
    echo "      (CUDA torch install failed; CPU torch from nemo will be used)"
  link_cuda_runtime_for_sherpa
else
  echo "      no nvidia-smi; skipping CUDA torch"
fi

echo
echo "Done. Try: wt --version  (or launch 'wt' from your applications menu)"
