"""
Diarization parameter sweep for sherpa-onnx.

Iterates over (embedding model, cluster-threshold, num-clusters,
min-duration-on, min-duration-off), runs sherpa-onnx-offline-speaker-diarization
on a set of test wavs with hand-labelled ground truth, and ranks configs by
diarization error rate (DER) computed at 10 ms frame resolution with optimal
speaker-label assignment (Hungarian matching).

Usage:
    python scripts/diar_sweep.py
"""

import csv
import dataclasses
import itertools
import os
import re
import subprocess
import sys
from pathlib import Path

try:
    from scipy.optimize import linear_sum_assignment
except ImportError:
    print("pip install scipy", file=sys.stderr)
    sys.exit(1)

# --- Paths ---
ROOT = Path(__file__).resolve().parent.parent
WT_DIR = Path(os.path.expandvars(r"%LOCALAPPDATA%\wt"))
SHERPA = WT_DIR / "sherpa-onnx-offline-speaker-diarization.exe"
SEG_MODEL = WT_DIR / "models" / "sherpa-onnx-pyannote-segmentation-3-0" / "model.onnx"
EMB_DIR = Path(r"C:\Users\asolo\src\wt\dist\embs")  # downloaded embedding models
TMP = Path(os.environ.get("TEMP", "/tmp"))

# --- Test set: (wav_path, ground_truth_segments) ---
# Ground truth: list of (start_sec, end_sec, speaker_label)
TEST_SET = {
    "s2_en": {
        "wav": TMP / "s2.wav",
        "src": Path(r"C:\Users\asolo\Desktop\3 speakers sample 2.mp4"),
        "dur": 35.04,
        "expected_speakers": 3,
        "truth": [
            (0.0, 11.16, "A"),
            (11.16, 24.48, "B"),
            (24.48, 30.19, "C"),
            (30.61, 35.04, "A"),
        ],
    },
    "s1_en": {
        "wav": TMP / "3spk.wav",
        "src": Path(r"C:\Users\asolo\Desktop\3_speakers_english.m4a"),
        "dur": 30.0,
        "expected_speakers": 3,
        "truth": [
            (0.0, 9.69, "A"),
            (9.69, 13.26, "C"),
            (13.26, 27.08, "A"),
            (27.08, 30.0, "C"),
        ],
    },
    "ru_2": {
        "wav": TMP / "ru.wav",
        "src": Path(r"C:\Users\asolo\Desktop\2_speakers_russian.m4a"),
        "dur": 27.16,
        "expected_speakers": 2,
        "truth": [
            (0.0, 8.35, "A"),
            (8.35, 26.63, "B"),
        ],
    },
}

# --- Sweep grid ---
EMBEDDINGS = sorted(p.name for p in EMB_DIR.glob("*.onnx"))
CLUSTER_THRESHOLDS = [0.45, 0.55, 0.65, 0.7, 0.75, 0.8, 0.85, 0.9]
NUM_CLUSTERS = [0]  # auto-only — we want best unsupervised quality
MIN_DURATION_ON = [0.2, 0.3, 0.5]
MIN_DURATION_OFF = [0.5]


@dataclasses.dataclass
class RunResult:
    wav: str
    emb: str
    threshold: float
    num_clusters: int
    min_on: float
    min_off: float
    detected: int
    segments: int
    der: float
    err: str = ""


def ensure_wavs():
    for k, v in TEST_SET.items():
        wav = v["wav"]
        if not wav.exists() or wav.stat().st_size < 1000:
            print(f"converting {v['src']} -> {wav}", file=sys.stderr)
            subprocess.run(
                ["ffmpeg", "-hide_banner", "-loglevel", "error", "-y",
                 "-i", str(v["src"]), "-ac", "1", "-ar", "16000", "-f", "wav",
                 str(wav)],
                check=True,
            )


SEG_RE = re.compile(r"^\s*([0-9]+\.[0-9]+)\s+--\s+([0-9]+\.[0-9]+)\s+speaker_(\d+)\s*$")


def run_sherpa(wav, emb, thr, num_clusters, min_on, min_off):
    args = [
        str(SHERPA),
        f"--segmentation.pyannote-model={SEG_MODEL}",
        f"--embedding.model={EMB_DIR / emb}",
        f"--min-duration-on={min_on}",
        f"--min-duration-off={min_off}",
    ]
    if num_clusters > 0:
        args.append(f"--clustering.num-clusters={num_clusters}")
    else:
        args.append(f"--clustering.cluster-threshold={thr}")
    args.append(str(wav))

    try:
        res = subprocess.run(args, capture_output=True, text=True, timeout=120)
    except subprocess.TimeoutExpired:
        return None, "timeout"
    if res.returncode != 0:
        return None, res.stderr.splitlines()[-1] if res.stderr else "exit_nonzero"
    segs = []
    for line in res.stdout.splitlines():
        m = SEG_RE.match(line)
        if m:
            segs.append((float(m[1]), float(m[2]), int(m[3])))
    return segs, ""


def frames_for(segs, dur, hop=0.01):
    """Return list of speaker label per frame; -1 = silence/unknown."""
    n = int(dur / hop) + 1
    out = [-1] * n
    for s, e, spk in segs:
        i0 = max(0, int(s / hop))
        i1 = min(n, int(e / hop))
        for i in range(i0, i1):
            out[i] = spk
    return out


def der(pred_segs, truth_segs, dur, hop=0.01):
    """Diarization error rate using Hungarian label matching.

    Frames where either side is silence count as miss/false alarm.
    Frames where both speak but mapping disagrees count as confusion.
    """
    pred = frames_for(pred_segs, dur, hop)
    truth_int = []
    label_map = {}
    for s, e, lab in truth_segs:
        if lab not in label_map:
            label_map[lab] = len(label_map)
    truth = [-1] * len(pred)
    for s, e, lab in truth_segs:
        i0 = max(0, int(s / hop))
        i1 = min(len(truth), int(e / hop))
        for i in range(i0, i1):
            truth[i] = label_map[lab]

    pred_labels = sorted({p for p in pred if p >= 0})
    truth_labels = sorted({t for t in truth if t >= 0})
    if not pred_labels or not truth_labels:
        return 1.0

    cost = [[0] * len(pred_labels) for _ in truth_labels]
    for t, p in zip(truth, pred):
        if t < 0 or p < 0:
            continue
        ti = truth_labels.index(t)
        pi = pred_labels.index(p)
        cost[ti][pi] -= 1  # negative for max-overlap as min-cost
    if len(pred_labels) < len(truth_labels):
        for row in cost:
            row.extend([0] * (len(truth_labels) - len(pred_labels)))
        pred_labels = pred_labels + [-99] * (len(truth_labels) - len(pred_labels))
    elif len(truth_labels) < len(pred_labels):
        for _ in range(len(pred_labels) - len(truth_labels)):
            cost.append([0] * len(pred_labels))
    row_ind, col_ind = linear_sum_assignment(cost)
    mapping = {pred_labels[c]: r for r, c in zip(row_ind, col_ind) if c < len(pred_labels)}

    total = 0
    err = 0
    for t, p in zip(truth, pred):
        if t < 0 and p < 0:
            continue
        total += 1
        if t < 0 or p < 0:
            err += 1
            continue
        if mapping.get(p, -1) != t:
            err += 1
    return err / max(total, 1)


def main():
    if not SHERPA.exists():
        sys.exit(f"sherpa exe missing: {SHERPA}")
    if not SEG_MODEL.exists():
        sys.exit(f"seg model missing: {SEG_MODEL}")
    if not EMBEDDINGS:
        sys.exit(f"no embeddings in {EMB_DIR}")
    ensure_wavs()

    grid = list(itertools.product(
        EMBEDDINGS, CLUSTER_THRESHOLDS, NUM_CLUSTERS, MIN_DURATION_ON, MIN_DURATION_OFF
    ))
    # If num_clusters > 0, threshold is ignored — dedupe.
    dedup = []
    seen = set()
    for emb, thr, nc, mon, moff in grid:
        key = (emb, thr if nc == 0 else None, nc, mon, moff)
        if key in seen:
            continue
        seen.add(key)
        dedup.append((emb, thr, nc, mon, moff))
    grid = dedup

    results = []
    total = len(grid) * len(TEST_SET)
    i = 0
    for emb, thr, nc, mon, moff in grid:
        for tname, tinfo in TEST_SET.items():
            i += 1
            print(f"[{i}/{total}] {tname} emb={emb} thr={thr} nc={nc} on={mon} off={moff}",
                  file=sys.stderr, flush=True)
            segs, err = run_sherpa(tinfo["wav"], emb, thr, nc, mon, moff)
            if segs is None:
                results.append(RunResult(tname, emb, thr, nc, mon, moff, 0, 0, 1.0, err))
                continue
            detected = len({s[2] for s in segs})
            d = der(segs, tinfo["truth"], dur=tinfo["dur"])
            results.append(RunResult(tname, emb, thr, nc, mon, moff, detected, len(segs), d))

    out_csv = ROOT / "scripts" / "diar_sweep_results.csv"
    with open(out_csv, "w", newline="", encoding="utf-8") as f:
        w = csv.writer(f)
        w.writerow(["wav", "emb", "threshold", "num_clusters", "min_on", "min_off",
                    "detected", "segments", "der", "err"])
        for r in results:
            w.writerow([r.wav, r.emb, r.threshold, r.num_clusters, r.min_on, r.min_off,
                        r.detected, r.segments, f"{r.der:.4f}", r.err])
    print(f"\nWrote {out_csv}\n", file=sys.stderr)

    # Aggregate: per-config mean DER + speaker-count penalty
    by_cfg = {}
    for r in results:
        if r.err:
            continue
        key = (r.emb, r.threshold if r.num_clusters == 0 else None,
               r.num_clusters, r.min_on, r.min_off)
        by_cfg.setdefault(key, []).append((r.wav, r.detected, r.der))

    rows = []
    for key, runs in by_cfg.items():
        if len(runs) < len(TEST_SET):
            continue
        emb, thr, nc, mon, moff = key
        avg_der = sum(d for _, _, d in runs) / len(runs)
        spk_penalty = 0.0
        for wav, det, _ in runs:
            exp = TEST_SET[wav]["expected_speakers"]
            spk_penalty += abs(det - exp) * 0.05
        score = avg_der + spk_penalty
        per_wav = " ".join(f"{w}:{d}spk/DER{dr:.2f}" for w, d, dr in runs)
        rows.append((score, avg_der, spk_penalty, emb, thr, nc, mon, moff, per_wav))

    rows.sort()
    print("\n=== TOP 20 CONFIGS ===")
    print(f"{'score':>6}  {'DER':>5}  {'spk±':>4}  emb  thr  nc  on  off  | runs")
    for sc, d, sp, emb, thr, nc, mon, moff, per_wav in rows[:20]:
        thr_s = f"{thr}" if thr is not None else "-"
        print(f"{sc:6.3f}  {d:5.3f}  {sp:4.2f}  {emb:30s}  thr={thr_s:>4}  nc={nc}  on={mon}  off={moff}  | {per_wav}")


if __name__ == "__main__":
    main()
