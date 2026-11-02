#!/bin/sh
# Build six CGO-free native binaries without cross C compilers.
set -eu
cd "$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
: "${GO:=go}"
: "${VERSION:=dev}"
case "$VERSION" in
    ''|[!A-Za-z0-9]*|*[!A-Za-z0-9._+-]*)
        printf 'Invalid VERSION: use letters, digits, dots, pluses or hyphens.\n' >&2
        exit 1
        ;;
esac
if [ "${#VERSION}" -gt 64 ]; then
    printf 'VERSION must be at most 64 characters.\n' >&2
    exit 1
fi
mkdir -p dist
for system in darwin linux windows; do
    for arch in amd64 arm64; do
        suffix=
        if [ "$system" = windows ]; then suffix=.exe; fi
        CGO_ENABLED=0 GOOS="$system" GOARCH="$arch" "$GO" build -trimpath \
            -ldflags "-s -w -X main.version=$VERSION" \
            -o "dist/syskline-$system-$arch$suffix" ./cmd/syskline
    done
done
: "${PYTHON:=python3}"
"$PYTHON" scripts/package_release.py "$VERSION"
printf 'Built macOS / Linux / Windows amd64 and arm64 binaries in dist/\n'
