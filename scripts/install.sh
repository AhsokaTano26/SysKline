#!/bin/sh
# Install an existing native binary. No network requests and no implicit sudo.
set -eu
: "${PREFIX:=$HOME/.local}"
binary=${1:-bin/syskline}
if [ ! -f "$binary" ] || [ ! -x "$binary" ]; then
    printf 'Executable not found: %s\nBuild first with make build.\n' "$binary" >&2
    exit 1
fi
mkdir -p "$PREFIX/bin"
install -m 755 "$binary" "$PREFIX/bin/syskline"
printf 'Installed %s/bin/syskline\nEnsure %s/bin is in PATH.\n' "$PREFIX" "$PREFIX"
