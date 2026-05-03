import argparse
import io
import json
import os
import sys
import tempfile

os.environ["HF_HUB_DISABLE_SYMLINKS_WARNING"] = "1"
os.environ["HF_HUB_DISABLE_SYMLINKS"] = "1"
os.environ.setdefault("PYTORCH_CUDA_ALLOC_CONF", "expandable_segments:True")

if sys.platform == "win32":
    _OrigTempDir = tempfile.TemporaryDirectory

    class _WinTempDir(_OrigTempDir):
        def __init__(self, *args, **kwargs):
            kwargs.setdefault("ignore_cleanup_errors", True)
            super().__init__(*args, **kwargs)

        def cleanup(self):
            try:
                super().cleanup()
            except (OSError, PermissionError):
                pass

        def __exit__(self, exc, value, tb):
            try:
                super().__exit__(exc, value, tb)
            except (OSError, PermissionError):
                pass

    tempfile.TemporaryDirectory = _WinTempDir


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


def _normalize_label(spk) -> str:
    s = str(spk)
    for prefix in ("speaker_", "SPEAKER_", "spk_", "SPK_"):
        if s.startswith(prefix):
            s = s[len(prefix):]
            break
    try:
        return f"SPEAKER_{int(s):02d}"
    except ValueError:
        return f"SPEAKER_{str(spk).upper()}"


def _parse_rttm(path: str):
    out = []
    with open(path, "r", encoding="utf-8") as f:
        for line in f:
            parts = line.strip().split()
            if len(parts) >= 8 and parts[0] == "SPEAKER":
                start = float(parts[3])
                dur = float(parts[4])
                spk = parts[7]
                out.append((start, start + dur, spk))
    return out


def main():
    parser = argparse.ArgumentParser(
        description="Speaker diarization (DiariZen wavlm-large-s80-md)"
    )
    parser.add_argument("wav", help="Path to WAV file (16kHz mono)")
    parser.add_argument("--num-speakers", type=int, default=0)
    args = parser.parse_args()

    real_stdout = os.dup(1)
    os.dup2(2, 1)
    sys.stdout = io.TextIOWrapper(os.fdopen(os.dup(2), "wb"), line_buffering=True)

    model_id = "BUT-FIT/diarizen-wavlm-large-s80-md"
    _fix_hf_symlinks(model_id)

    try:
        import torch
    except ImportError:
        print("torch not installed", file=sys.stderr, flush=True)
        os.dup2(real_stdout, 1)
        sys.stdout = io.TextIOWrapper(os.fdopen(1, "wb"), line_buffering=True)
        sys.stdout.write("[]\n")
        sys.stdout.flush()
        sys.exit(1)

    _orig_load = torch.load

    def _patched_load(*a, **kw):
        kw["weights_only"] = False
        return _orig_load(*a, **kw)

    torch.load = _patched_load

    try:
        from diarizen.pipelines.inference import DiariZenPipeline
    except ImportError:
        print(
            "diarizen not installed. Install with: uv pip install git+https://github.com/BUTSpeechFIT/DiariZen.git",
            file=sys.stderr,
            flush=True,
        )
        os.dup2(real_stdout, 1)
        sys.stdout = io.TextIOWrapper(os.fdopen(1, "wb"), line_buffering=True)
        sys.stdout.write("[]\n")
        sys.stdout.flush()
        sys.exit(1)

    force_cpu = os.environ.get("WT_DIAR_DEVICE") == "cpu"
    device = "cpu" if force_cpu else ("cuda" if torch.cuda.is_available() else "cpu")
    print(f"device: {device}", file=sys.stderr, flush=True)
    print("loading model...", file=sys.stderr, flush=True)

    try:
        pipeline = DiariZenPipeline.from_pretrained(
            model_id, use_auth_token=os.environ.get("HF_TOKEN")
        )
    except TypeError:
        pipeline = DiariZenPipeline.from_pretrained(model_id)

    try:
        if device == "cuda" and hasattr(pipeline, "to"):
            pipeline.to(torch.device("cuda"))
    except Exception as e:
        print(f"pipeline.to(cuda) failed: {e}", file=sys.stderr, flush=True)

    for _attr in ("_segmentation", "_embedding"):
        _obj = getattr(pipeline, _attr, None)
        if _obj is not None and hasattr(_obj, "batch_size"):
            try:
                _obj.batch_size = 4
            except Exception:
                pass
    for _attr in (
        "embedding_batch_size",
        "segmentation_batch_size",
        "seg_batch_size",
        "embed_batch_size",
    ):
        if hasattr(pipeline, _attr):
            try:
                setattr(pipeline, _attr, 4)
            except Exception:
                pass

    print("processing...", file=sys.stderr, flush=True)

    pipeline_kwargs = {}
    if args.num_speakers > 0:
        try:
            import inspect

            sig = inspect.signature(pipeline.__call__)
            if "num_speakers" in sig.parameters:
                pipeline_kwargs["num_speakers"] = args.num_speakers
        except (TypeError, ValueError):
            pass

    try:
        result = pipeline(args.wav, **pipeline_kwargs)
    except torch.OutOfMemoryError as e:
        if device != "cpu":
            print(f"CUDA OOM ({e}); retrying on CPU...", file=sys.stderr, flush=True)
            del pipeline
            torch.cuda.empty_cache()
            device = "cpu"
            try:
                pipeline = DiariZenPipeline.from_pretrained(
                    model_id, use_auth_token=os.environ.get("HF_TOKEN")
                )
            except TypeError:
                pipeline = DiariZenPipeline.from_pretrained(model_id)
            result = pipeline(args.wav, **pipeline_kwargs)
        else:
            raise

    os.dup2(real_stdout, 1)
    sys.stdout = io.TextIOWrapper(os.fdopen(1, "wb"), line_buffering=True)

    output = []
    speakers = set()

    if isinstance(result, str) and os.path.isfile(result):
        for start, end, spk in _parse_rttm(result):
            label = _normalize_label(spk)
            speakers.add(label)
            output.append(
                {"start": round(start, 3), "end": round(end, 3), "speaker": label}
            )
    elif hasattr(result, "itertracks"):
        for turn, _, spk in result.itertracks(yield_label=True):
            label = _normalize_label(spk)
            speakers.add(label)
            output.append(
                {
                    "start": round(float(turn.start), 3),
                    "end": round(float(turn.end), 3),
                    "speaker": label,
                }
            )
    else:
        for item in result or []:
            if isinstance(item, (list, tuple)) and len(item) >= 3:
                start, end, spk = float(item[0]), float(item[1]), item[2]
                label = _normalize_label(spk)
                speakers.add(label)
                output.append(
                    {"start": round(start, 3), "end": round(end, 3), "speaker": label}
                )

    output.sort(key=lambda s: s["start"])

    if args.num_speakers > 0 and "num_speakers" not in pipeline_kwargs:
        print(
            f"note: DiariZen API did not accept num_speakers={args.num_speakers}; using estimated count",
            file=sys.stderr,
            flush=True,
        )

    print(
        f"done: {len(speakers)} speakers, {len(output)} segments",
        file=sys.stderr,
        flush=True,
    )

    json.dump(output, sys.stdout, indent=None)
    sys.stdout.write("\n")
    sys.stdout.flush()


if __name__ == "__main__":
    try:
        main()
    except SystemExit:
        raise
    except Exception as e:
        print(f"diarize_diarizen failed: {type(e).__name__}: {e}", file=sys.stderr, flush=True)
        try:
            sys.stdout.write("[]\n")
            sys.stdout.flush()
        except Exception:
            pass
        sys.exit(1)
