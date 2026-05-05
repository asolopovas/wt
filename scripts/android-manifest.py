#!/usr/bin/env python3
"""Render cmd/wt-gui/AndroidManifest.xml from the .in template."""
import sys

ver, app_name = sys.argv[1], sys.argv[2]
if ver.startswith('dev-'):
    import datetime
    digits = [int(x) for x in ver[4:].split('-') if x.isdigit()]
    digits = (digits + [0] * 6)[:6]
    y, mo, d, hh, mm, ss = digits
    try:
        ts = datetime.datetime(y, mo, d, hh, mm, ss, tzinfo=datetime.timezone.utc)
    except ValueError:
        ts = datetime.datetime.now(datetime.timezone.utc)
    epoch = datetime.datetime(2024, 1, 1, tzinfo=datetime.timezone.utc)
    code = max(1, int((ts - epoch).total_seconds()) // 60)
else:
    parts = (ver.split('.') + ['0', '0', '0'])[:3]
    maj, mn, pat = (int(x) if x.isdigit() else 0 for x in parts)
    code = maj * 10000 + mn * 100 + pat

with open('cmd/wt-gui/AndroidManifest.xml.in', 'r', encoding='utf-8') as f:
    data = f.read()
data = (data
        .replace('@@VERSION_NAME@@', ver)
        .replace('@@VERSION_CODE@@', str(code))
        .replace('@@APP_NAME@@', app_name)
        .replace('@@DEBUG@@', 'true'))
with open('cmd/wt-gui/AndroidManifest.xml', 'w', encoding='utf-8') as f:
    f.write(data)
