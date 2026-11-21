#!/bin/sh
set -eu

version=$(awk '$1 == "go" { print $2; exit }' go.mod)
if [ -z "$version" ]; then
    echo 'go.mod is missing a Go version' >&2
    exit 1
fi

# Go 1.21 and later name the initial stable toolchain go1.N.0.
case "$version" in
    *.*.*|*beta*|*rc*) ;;
    *) version="${version}.0" ;;
esac

go env -w "GOTOOLCHAIN=go${version}"
