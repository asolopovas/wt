#!/usr/bin/env python3
"""Render cmd/wt-gui/AndroidManifest.xml from the .in template."""
import sys

ver, app_name = sys.argv[1], sys.argv[2]
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
