#!/bin/sh
# Build CGO-free Linux binaries without a cross C compiler.
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
stage=$(mktemp -d "${TMPDIR:-/tmp}/syskline-package.XXXXXX")
trap 'rm -rf "$stage"' 0
trap 'exit 1' 1 2 15
mkdir -p "$stage/configs"
cp LICENSE README.md "$stage/"
cp configs/config.example.toml "$stage/configs/"
for arch in amd64 arm64; do
    CGO_ENABLED=0 GOOS=linux GOARCH="$arch" "$GO" build -trimpath \
        -ldflags "-s -w -X main.version=$VERSION" \
        -o "dist/syskline-linux-$arch" ./cmd/syskline
    install -m 755 "dist/syskline-linux-$arch" "$stage/syskline"
    COPYFILE_DISABLE=1 tar -czf "dist/syskline_${VERSION}_linux_${arch}.tar.gz" -C "$stage" \
        syskline LICENSE README.md configs/config.example.toml
done
cd dist
if command -v sha256sum >/dev/null 2>&1; then
    sha256sum syskline-linux-amd64 syskline-linux-arm64 \
        "syskline_${VERSION}_linux_amd64.tar.gz" "syskline_${VERSION}_linux_arm64.tar.gz" > SHA256SUMS
else
    shasum -a 256 syskline-linux-amd64 syskline-linux-arm64 \
        "syskline_${VERSION}_linux_amd64.tar.gz" "syskline_${VERSION}_linux_arm64.tar.gz" > SHA256SUMS
fi
printf 'Built Linux amd64 / arm64 binaries and %s packages in dist/\n' "$VERSION"
