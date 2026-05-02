#!/usr/bin/env bash
# Publish a release: push HEAD + tag, create GH release if missing, upload artifacts.
# Extracted from Taskfile because mvdan/sh (Task's embedded shell on Windows)
# silently aborts the script after `git push origin <tag>`, never reaching the
# gh-release commands. msys64 bash runs the full flow correctly.
#
# Usage: bash scripts/release-publish.sh <version>
set -euo pipefail

ver="${1:?version required}"
tag="v$ver"
echo "=== Releasing $tag ==="

echo "--- git push HEAD ---"
git push origin HEAD

if git rev-parse "$tag" >/dev/null 2>&1; then
  echo "--- tag $tag already exists locally ---"
else
  echo "--- creating tag $tag ---"
  git tag -a "$tag" -m "Release $tag"
fi
echo "--- git push tag ---"
git push origin "$tag"

artifacts=()
for f in \
  "dist/wt-setup-$ver.exe" \
  "dist/wt-$ver.apk" \
  "dist/wt_${ver}_amd64.deb"; do
  if [ -f "$f" ]; then
    artifacts+=("$f")
  fi
done
echo "--- artifacts: ${artifacts[*]} ---"
if [ "${#artifacts[@]}" -eq 0 ]; then
  echo "ERROR: no artifacts found to upload" >&2
  exit 1
fi

view_rc=0
gh release view "$tag" >/dev/null 2>&1 || view_rc=$?
echo "--- gh release view rc=$view_rc ---"
if [ "$view_rc" -ne 0 ]; then
  echo "--- gh release create $tag ---"
  gh release create "$tag" --title "$tag" --generate-notes
fi

echo "--- gh release upload $tag ---"
bash scripts/gh-release-upload.sh "$tag" "${artifacts[@]}"
echo "=== Released $tag: https://github.com/asolopovas/wt/releases/tag/$tag ==="
