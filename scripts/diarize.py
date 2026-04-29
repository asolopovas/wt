import argparse
import io
import json
import os
import sys

os.environ["HF_HUB_DISABLE_SYMLINKS_WARNING"] = "1"
os.environ["HF_HUB_DISABLE_SYMLINKS"] = "1"
os.environ.setdefault("PYTORCH_CUDA_ALLOC_CONF", "expandable_segments:True")


def _fix_hf_symlinks(model_id: str) -> None:
    cache_dir = os.path.join(os.path.expanduser("~"), ".cache", "huggingface", "hub")
    model_dir = os.path.join(cache_dir, "models--" + model_id.replace("/", "--"))
    snapshots = os.path.join(model_dir, "snapshots")
    blobs = os.path.join(model_dir, "blobs")
    if not os.path.isdir(snapshots) or not os.path.isdir(blobs):
        return
    for root, _dirs, files in os.walk(snapshots):
        for fname in files:
            fpath = os.path.join(root, fname)
            if os.path.islink(fpath) and not os.path.exists(fpath):
                target = os.path.realpath(fpath)
                if os.path.exists(target):
                    os.remove(fpath)
                    import shutil

                    shutil.copy2(target, fpath)
                    print(f"fixed broken symlink: {fname}", file=sys.stderr, flush=True)


def main():
    parser = argparse.ArgumentParser(
        description="Speaker diarization (NeMo Sortformer)"
    )
    parser.add_argument("wav", help="Path to WAV file (16kHz mono)")
    parser.add_argument("--num-speakers", type=int, default=0)
    args = parser.parse_args()

    real_stdout = os.dup(1)
    os.dup2(2, 1)
    sys.stdout = io.TextIOWrapper(os.fdopen(os.dup(2), "wb"), line_buffering=True)

    _fix_hf_symlinks("nvidia/diar_sortformer_4spk-v1")

    try:
        from nemo.collections.asr.models import SortformerEncLabelModel
    except ImportError:
        print(
            "nemo_toolkit not installed. Install with: uv pip install 'nemo_toolkit[asr]'",
            file=sys.stderr,
        )
        sys.exit(1)

    import torch

    force_cpu = os.environ.get("WT_DIAR_DEVICE") == "cpu"
    device = "cpu" if force_cpu else ("cuda" if torch.cuda.is_available() else "cpu")
    print(f"device: {device}", file=sys.stderr, flush=True)
    print("loading model...", file=sys.stderr, flush=True)

    def _load(dev):
        m = SortformerEncLabelModel.from_pretrained(
            "nvidia/diar_sortformer_4spk-v1", map_location=dev
        )
        m.eval()
        return m

    model = _load(device)

    print("processing...", file=sys.stderr, flush=True)

    if args.num_speakers > 0:
        print(
            f"note: SortformerEncLabelModel is fixed at 4 speakers; ignoring --num-speakers={args.num_speakers}",
            file=sys.stderr,
            flush=True,
        )
    try:
        result = model.diarize(audio=args.wav, batch_size=1)
    except torch.OutOfMemoryError as e:
        if device != "cpu":
            print(
                f"CUDA OOM ({e}); retrying on CPU...",
                file=sys.stderr,
                flush=True,
            )
            del model
            torch.cuda.empty_cache()
            device = "cpu"
            model = _load(device)
            result = model.diarize(audio=args.wav, batch_size=1)
        else:
            raise

    os.dup2(real_stdout, 1)
    sys.stdout = io.TextIOWrapper(os.fdopen(1, "wb"), line_buffering=True)

    output = []
    speakers = set()
    for seg_list in result:
        for seg in seg_list:
            parts = seg.split()
            if len(parts) == 3:
                start, end, spk = float(parts[0]), float(parts[1]), parts[2]
                spk_clean = spk.replace("speaker_", "")
                try:
                    label = f"SPEAKER_{int(spk_clean):02d}"
                except ValueError:
                    label = spk.upper()
                speakers.add(label)
                output.append(
                    {"start": round(start, 3), "end": round(end, 3), "speaker": label}
                )

    output.sort(key=lambda s: s["start"])

    print(
        f"done: {len(speakers)} speakers, {len(output)} segments",
        file=sys.stderr,
        flush=True,
    )

    json.dump(output, sys.stdout, indent=None)
    sys.stdout.write("\n")
    sys.stdout.flush()


if __name__ == "__main__":
    main()
