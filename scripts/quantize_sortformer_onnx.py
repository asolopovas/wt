"""Convert exported Sortformer ONNX to FP16 (smaller, parity-preserving).

Input:  dist/models/sortformer-4spk-v1.onnx       (FP32, ~490 MB)
Output: dist/models/sortformer-4spk-v1.fp16.onnx  (~245 MB)

INT8 dynamic quantization breaks Sortformer's attention path (mean diff
~0.16, false speakers detected). FP16 preserves output quality while
halving model size.

Usage:
    PYTHONUTF8=1 uv run --python 3.12 \
        --with onnx --with onnxconverter-common \
        python scripts/quantize_sortformer_onnx.py
"""

import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
SRC = ROOT / "dist" / "models" / "sortformer-4spk-v1.onnx"
DST = ROOT / "dist" / "models" / "sortformer-4spk-v1.fp16.onnx"


def main() -> int:
    if not SRC.exists():
        print(f"missing {SRC}; run export_sortformer_onnx.py first", file=sys.stderr)
        return 1

    import onnx
    from onnxconverter_common import float16

    print(f"loading {SRC}...", file=sys.stderr)
    model = onnx.load(str(SRC))

    print("converting to FP16 (keep IO in FP32)...", file=sys.stderr)
    model_fp16 = float16.convert_float_to_float16(
        model,
        keep_io_types=True,
        disable_shape_infer=True,
    )

    print(f"saving {DST}...", file=sys.stderr)
    onnx.save(model_fp16, str(DST))

    src_mb = SRC.stat().st_size / (1024 * 1024)
    dst_mb = DST.stat().st_size / (1024 * 1024)
    print(f"done: {DST} ({dst_mb:.1f} MB, was {src_mb:.1f} MB)", file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main())
