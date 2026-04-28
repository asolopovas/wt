"""Export NVIDIA Sortformer 4spk-v1 to ONNX.

Run on a desktop where NeMo + torch are already installed (the same env
used by diarize.py). Produces dist/models/sortformer-4spk-v1.onnx.

Usage:
    uv run python scripts/export_sortformer_onnx.py
    # or:
    python scripts/export_sortformer_onnx.py
"""

import os
import sys
from pathlib import Path

os.environ["HF_HUB_DISABLE_SYMLINKS_WARNING"] = "1"
os.environ["HF_HUB_DISABLE_SYMLINKS"] = "1"

OUT_DIR = Path(__file__).resolve().parent.parent / "dist" / "models"
OUT_PATH = OUT_DIR / "sortformer-4spk-v1.onnx"


def main() -> int:
    OUT_DIR.mkdir(parents=True, exist_ok=True)

    try:
        from nemo.collections.asr.models import SortformerEncLabelModel
    except ImportError:
        print(
            "nemo_toolkit not installed. Install with: uv pip install 'nemo_toolkit[asr]'",
            file=sys.stderr,
        )
        return 1

    import torch

    device = "cuda" if torch.cuda.is_available() else "cpu"
    print(f"device: {device}", file=sys.stderr)
    print("loading nvidia/diar_sortformer_4spk-v1...", file=sys.stderr)
    model = SortformerEncLabelModel.from_pretrained(
        "nvidia/diar_sortformer_4spk-v1", map_location=device
    )
    model.eval()

    print(f"exporting -> {OUT_PATH}", file=sys.stderr)
    model.export(str(OUT_PATH))

    size_mb = OUT_PATH.stat().st_size / (1024 * 1024)
    print(f"done: {OUT_PATH} ({size_mb:.1f} MB)", file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main())
