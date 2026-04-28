"""Export NVIDIA Sortformer 4spk-v1 to ONNX (features-in wrapper).

torch.onnx can't export the STFT inside the NeMo mel preprocessor, so we
expose only the post-preprocessor path: the wrapper takes log-mel
features [B, 80, T] + lengths and returns 4-speaker per-frame
probabilities [B, T_frames, 4]. Mel features are computed in Go.

Usage:
    PYTHONUTF8=1 uv run --python 3.12 \
        --with "nemo_toolkit[asr]" --with torch \
        --with onnxscript --with onnx \
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
        print("nemo_toolkit[asr] not installed.", file=sys.stderr)
        return 1

    import torch
    import torch.nn as nn

    print("loading nvidia/diar_sortformer_4spk-v1...", file=sys.stderr)
    model = SortformerEncLabelModel.from_pretrained(
        "nvidia/diar_sortformer_4spk-v1", map_location="cpu"
    )
    model.eval()

    fe = model.preprocessor.featurizer
    print(
        f"feature config: sample_rate=16000 n_fft={fe.n_fft} "
        f"win_length={fe.win_length} hop_length={fe.hop_length} "
        f"nfilt={fe.nfilt}",
        file=sys.stderr,
    )

    class FeatWrapper(nn.Module):
        def __init__(self, m):
            super().__init__()
            self.m = m

        def forward(self, processed_signal, processed_signal_length):
            emb_seq, emb_seq_length = self.m.frontend_encoder(
                processed_signal=processed_signal,
                processed_signal_length=processed_signal_length,
            )
            return self.m.forward_infer(emb_seq, emb_seq_length)

    wrapper = FeatWrapper(model).eval()

    # 30 s @ 16 kHz: T_features = 1 + (samples - win_length) // hop_length
    # = 1 + (480000 - 400) // 160 = 1 + 2997 = 2998
    feats = torch.randn(1, 80, 2998, dtype=torch.float32)
    feats_len = torch.tensor([2998], dtype=torch.int64)

    print("dry-run forward...", file=sys.stderr)
    with torch.no_grad():
        out = wrapper(feats, feats_len)
    print(f"  out.shape={tuple(out.shape)} dtype={out.dtype}", file=sys.stderr)

    print(f"exporting -> {OUT_PATH}", file=sys.stderr)
    torch.onnx.export(
        wrapper,
        (feats, feats_len),
        str(OUT_PATH),
        input_names=["processed_signal", "processed_signal_length"],
        output_names=["preds"],
        dynamic_axes={
            "processed_signal": {0: "batch", 2: "frames"},
            "processed_signal_length": {0: "batch"},
            "preds": {0: "batch", 1: "out_frames"},
        },
        opset_version=17,
        do_constant_folding=True,
        dynamo=False,
    )

    size_mb = OUT_PATH.stat().st_size / (1024 * 1024)
    print(f"done: {OUT_PATH} ({size_mb:.1f} MB)", file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main())
