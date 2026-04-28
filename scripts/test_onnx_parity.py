"""Compare FP32 vs FP16 ONNX Sortformer outputs on a real audio file.

Computes log-mel features via the NeMo preprocessor (so we test only the
encoder/decoder ONNX path), runs both ONNX models, and reports max abs
diff in per-frame speaker probabilities.

Usage:
    PYTHONUTF8=1 uv run --python 3.12 \
        --with "nemo_toolkit[asr]" --with torch \
        --with onnx --with onnxruntime --with soundfile \
        python scripts/test_onnx_parity.py samples/two_people_chat.opus
"""

import os
import subprocess
import sys
import tempfile
from pathlib import Path

os.environ["HF_HUB_DISABLE_SYMLINKS_WARNING"] = "1"
os.environ["HF_HUB_DISABLE_SYMLINKS"] = "1"

ROOT = Path(__file__).resolve().parent.parent
FP32 = ROOT / "dist" / "models" / "sortformer-4spk-v1.onnx"
FP16_PATH = ROOT / "dist" / "models" / "sortformer-4spk-v1.fp16.onnx"


def to_wav_16k(src: Path) -> Path:
    out = Path(tempfile.gettempdir()) / (src.stem + ".16k.wav")
    subprocess.run(
        ["ffmpeg", "-y", "-i", str(src), "-ac", "1", "-ar", "16000", "-f", "wav", str(out)],
        check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
    )
    return out


def main() -> int:
    if len(sys.argv) < 2:
        print("usage: test_onnx_parity.py <audio>", file=sys.stderr)
        return 1
    audio_in = Path(sys.argv[1])

    import numpy as np
    import soundfile as sf
    import torch
    import onnxruntime as ort
    from nemo.collections.asr.models import SortformerEncLabelModel

    wav_path = to_wav_16k(audio_in)
    audio, sr = sf.read(wav_path)
    audio = audio.astype(np.float32)
    print(f"audio: {audio.shape[0]/sr:.1f}s @ {sr} Hz", file=sys.stderr)

    print("loading NeMo preprocessor for log-mel...", file=sys.stderr)
    m = SortformerEncLabelModel.from_pretrained(
        "nvidia/diar_sortformer_4spk-v1", map_location="cpu"
    )
    m.eval()

    x = torch.from_numpy(audio).unsqueeze(0)
    xl = torch.tensor([audio.shape[0]], dtype=torch.int64)
    with torch.no_grad():
        feats, feats_len = m.process_signal(audio_signal=x, audio_signal_length=xl)
    feats = feats[:, :, : feats_len.max()]
    feats_np = feats.numpy().astype(np.float32)
    feats_len_np = feats_len.numpy().astype(np.int64)
    print(f"feats: {feats_np.shape}, len={feats_len_np.tolist()}", file=sys.stderr)

    sess_opts = ort.SessionOptions()
    sess_opts.intra_op_num_threads = 4

    def run(model_path):
        sess = ort.InferenceSession(str(model_path), sess_opts, providers=["CPUExecutionProvider"])
        outs = sess.run(["preds"], {
            "processed_signal": feats_np,
            "processed_signal_length": feats_len_np,
        })
        return outs[0]

    print("running FP32...", file=sys.stderr)
    p32 = run(FP32)
    print(f"  preds shape: {p32.shape}", file=sys.stderr)

    print("running FP16...", file=sys.stderr)
    p8 = run(FP16)
    print(f"  preds shape: {p8.shape}", file=sys.stderr)

    diff = np.abs(p32 - p8)
    print(f"max abs diff: {diff.max():.4f}", file=sys.stderr)
    print(f"mean abs diff: {diff.mean():.4f}", file=sys.stderr)

    fp32_speakers = (p32[0] > 0.5).any(axis=0).sum()
    int8_speakers = (p8[0] > 0.5).any(axis=0).sum()
    print(f"FP32 active speakers (thr 0.5): {fp32_speakers}", file=sys.stderr)
    print(f"FP16 active speakers (thr 0.5): {int8_speakers}", file=sys.stderr)
    return 0


if __name__ == "__main__":
    sys.exit(main())
