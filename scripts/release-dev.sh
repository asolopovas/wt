#!/usr/bin/env bash
set -euo pipefail

ver="${1:?version required}"
sha="$(git rev-parse --short HEAD)"
branch="$(git rev-parse --abbrev-ref HEAD)"
[ "$branch" = "HEAD" ] && branch="main"
tag="dev"
devdir="dist/dev"

echo "Updating $tag to $sha (branch $branch, version $ver)"

git push origin HEAD
git tag -f "$tag" HEAD
git push origin "$tag" --force

rm -rf "$devdir"
mkdir -p "$devdir"

artifacts=()
if [ -f "dist/wt-setup-$ver.exe" ]; then
	cp "dist/wt-setup-$ver.exe" "$devdir/wt-setup-$branch.exe"
	artifacts+=("$devdir/wt-setup-$branch.exe")
fi
if [ -f "dist/wt-$ver.apk" ]; then
	cp "dist/wt-$ver.apk" "$devdir/wt-$branch.apk"
	artifacts+=("$devdir/wt-$branch.apk")
fi
if [ -f "dist/wt_${ver}_amd64.deb" ]; then
	cp "dist/wt_${ver}_amd64.deb" "$devdir/wt-${branch}_amd64.deb"
	artifacts+=("$devdir/wt-${branch}_amd64.deb")
fi

if [ "${#artifacts[@]}" -eq 0 ]; then
	echo "ERROR: no artifacts found to upload" >&2
	exit 1
fi
echo "Artifacts: ${artifacts[*]}"

if gh release view "$tag" >/dev/null 2>&1; then
	echo "Deleting existing $tag release..."
	gh release delete "$tag" --yes --cleanup-tag=false
fi

echo "Creating $tag release..."
gh release create "$tag" \
	--title "Dev ($branch @ $sha)" \
	--prerelease \
	--notes "Dev build of $branch. Commit \`$sha\`, version $ver. Updated automatically; not a stable release."

bash "$(dirname "$0")/gh-release-upload.sh" "$tag" "${artifacts[@]}"
echo "Dev release: https://github.com/asolopovas/wt/releases/tag/$tag"
