#!/usr/bin/env python3
"""Patch the Fyne-built APK with native libs, sherpa models, llama-cli, and the foreground-service dex.

Args (env-style): APK, LIBCXX, LIBOMP, SHERPA_BIN, SHERPA_SEG, SHERPA_EMB, LLAMA_BIN, SVC_DEX, OUT
"""
import os
import sys
import zipfile

required = ['APK', 'LIBCXX', 'LIBOMP', 'OUT']
missing = [k for k in required if not os.environ.get(k)]
if missing:
    print(f"ERROR: missing env: {missing}", file=sys.stderr)
    sys.exit(1)

apk = os.environ['APK']
libcxx = os.environ['LIBCXX'].replace('\\', '/')
libomp = os.environ['LIBOMP'].replace('\\', '/')
sherpa_bin = os.environ.get('SHERPA_BIN', '')
sherpa_seg = os.environ.get('SHERPA_SEG', '')
sherpa_emb = os.environ.get('SHERPA_EMB', '')
llama_bin = os.environ.get('LLAMA_BIN', '')
ffmpeg_bin = os.environ.get('FFMPEG_BIN', '')
svc_dex = os.environ.get('SVC_DEX', '')
out = os.environ['OUT']

with zipfile.ZipFile(apk, 'r') as zin, zipfile.ZipFile(out, 'w', zipfile.ZIP_DEFLATED) as zout:
    for item in zin.infolist():
        if item.filename.startswith('META-INF/'):
            continue
        zout.writestr(item, zin.read(item.filename))
    zout.write(libcxx, 'lib/arm64-v8a/libc++_shared.so', compress_type=zipfile.ZIP_STORED)
    zout.write(libomp, 'lib/arm64-v8a/libomp.so', compress_type=zipfile.ZIP_STORED)
    if sherpa_bin and os.path.exists(sherpa_bin):
        zout.write(sherpa_bin, 'lib/arm64-v8a/libsherpa-diar.so', compress_type=zipfile.ZIP_STORED)
    if sherpa_seg and os.path.exists(sherpa_seg):
        zout.write(sherpa_seg, 'assets/sherpa-models/seg.onnx', compress_type=zipfile.ZIP_STORED)
    if sherpa_emb and os.path.exists(sherpa_emb):
        zout.write(sherpa_emb, 'assets/sherpa-models/emb.onnx', compress_type=zipfile.ZIP_STORED)
    if llama_bin and os.path.exists(llama_bin):
        zout.write(llama_bin, 'lib/arm64-v8a/libllama-cli.so', compress_type=zipfile.ZIP_STORED)
    if ffmpeg_bin and os.path.exists(ffmpeg_bin):
        zout.write(ffmpeg_bin, 'lib/arm64-v8a/libffmpeg.so', compress_type=zipfile.ZIP_STORED)
    if svc_dex and os.path.exists(svc_dex):
        zout.write(svc_dex, 'classes2.dex', compress_type=zipfile.ZIP_DEFLATED)

print('Patched:', os.path.getsize(out))
