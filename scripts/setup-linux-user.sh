#!/usr/bin/env bash
# Per-user wt setup. Installed by the .deb at /opt/wt/wt-setup
# (also exposed as /usr/bin/wt-setup). Runs three things:
#   1. Symlinks bundled models from /opt/wt/models into ~/.config/wt/models
#   2. Creates ~/.config/wt/python venv with nemo_toolkit[asr]
#   3. Optionally installs torch with CUDA wheels when nvidia-smi is present
set -euo pipefail

UV="/opt/wt/uv"
BUNDLED_MODELS="/opt/wt/models"
cfgdir="${XDG_CONFIG_HOME:-$HOME/.config}/wt"
modelsdir="$cfgdir/models"
pydir="$cfgdir/python"
mkdir -p "$cfgdir" "$modelsdir"

echo "[1/3] Linking bundled models -> $modelsdir"
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

echo "[2/3] Creating Python 3.12 venv at $pydir"
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

echo "[3/3] CUDA torch (optional)"
if command -v nvidia-smi >/dev/null 2>&1; then
  echo "      NVIDIA GPU detected; installing torch with CUDA..."
  "$UV" pip install torch torchvision torchaudio \
    --index-url https://download.pytorch.org/whl/cu124 --python "$VPY" || \
    echo "      (CUDA torch install failed; CPU torch from nemo will be used)"
else
  echo "      no nvidia-smi; skipping CUDA torch"
fi

echo
echo "Done. Try: wt --version  (or launch 'wt' from your applications menu)"
