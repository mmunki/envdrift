#!/usr/bin/env bash
# Build the release binaries for every supported target and package them.
# Output goes to dist/ (or $OUT) and is never committed.
set -euo pipefail

cd "$(dirname "$0")/.."

VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"
OUT="${OUT:-dist}"

rm -rf "$OUT"
mkdir -p "$OUT"

targets=(
  "linux amd64"
  "linux arm64"
  "darwin amd64"
  "darwin arm64"
  "windows amd64"
  "windows arm64"
)

for target in "${targets[@]}"; do
  read -r goos goarch <<<"$target"
  name="envdrift-${goos}-${goarch}"
  bin="$OUT/$name"
  if [ "$goos" = windows ]; then
    bin="$bin.exe"
  fi

  echo "==> building $goos/$goarch (version $VERSION)"
  GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 \
    go build -trimpath -ldflags "-s -w -X main.version=$VERSION" \
    -o "$bin" ./cmd/envdrift

  if [ "$goos" = windows ]; then
    (cd "$OUT" && zip -q "$name.zip" "$(basename "$bin")" && rm -f "$(basename "$bin")")
  else
    tar -C "$OUT" -czf "$OUT/$name.tar.gz" "$(basename "$bin")"
    rm -f "$bin"
  fi
done

(cd "$OUT" && sha256sum envdrift-* > checksums.txt)

echo "==> artefacts in $OUT"
ls -l "$OUT"
