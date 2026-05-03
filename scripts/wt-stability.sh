#!/usr/bin/env bash
# wt-stability — pull recent crash + lmkd-kill events for the wt app
# from the connected Android device and print a structured report.
#
# Usage: bash scripts/wt-stability.sh [SINCE_MIN]   (default 30 min)
set -euo pipefail
SINCE_MIN="${1:-30}"
PKG="com.asolopovas.wtranscribe"

echo "== wt stability report (last ${SINCE_MIN} min) =="
echo

echo "-- lmkd kills --"
adb logcat -d -t "${SINCE_MIN}m" 2>/dev/null \
  | grep -E "lmkd.*${PKG}|ActivityManager.*${PKG}.*died" \
  | tail -10 || echo "  (none)"
echo

echo "-- native tombstones (dropbox) --"
# dumpsys dropbox --print emits one block per crash; first line of each
# block is the timestamp + tag (e.g. "system_app_native_crash").
adb shell "dumpsys dropbox --print 2>/dev/null" \
  | awk -v pkg="$PKG" 'BEGIN{p=0} /^=====/ {p=0} /^[0-9-]{10} [0-9:.]+ [A-Z] [a-z_]+_(native_)?crash/ {hdr=$0; p=0} $0 ~ pkg {p=1; print "  " hdr; print "  " $0}' \
  | tail -20 || echo "  (none)"
echo

echo "-- foreground OOM scores (right now) --"
adb shell "pidof ${PKG}" 2>/dev/null | awk '{print $1}' | while read -r pid; do
  [ -z "$pid" ] && continue
  echo "  pid=$pid"
  adb shell "cat /proc/${pid}/oom_score /proc/${pid}/oom_score_adj /proc/${pid}/status | grep -E 'VmRSS|VmSwap|Threads'" 2>/dev/null | sed 's/^/    /'
done || echo "  (not running)"
echo

echo "-- system memory pressure --"
adb shell 'cat /proc/pressure/memory 2>/dev/null' | sed 's/^/  /' || true
