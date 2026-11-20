#!/usr/bin/env bash
# Локальная сборка macOS-версий CraftPull: Intel, Apple Silicon и universal.
#
# Собирать нужно на macOS: графическая часть (Fyne) требует cgo и системных
# библиотек, кросс-компиляция с Windows невозможна.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

OUT="dist"
mkdir -p "$OUT"

VERSION="$(grep -oE 'Version = "[^"]+"' internal/buildinfo/buildinfo.go | head -n1 | cut -d'"' -f2)"
echo "==> CraftPull $VERSION · $(go version)"

echo "==> Сборка для Intel (darwin/amd64)"
CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w" -o "$OUT/craftpull-darwin-amd64" ./cmd/craftpull

echo "==> Сборка для Apple Silicon (darwin/arm64)"
CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -ldflags "-s -w" -o "$OUT/craftpull-darwin-arm64" ./cmd/craftpull

echo "==> Universal-бинарник (lipo)"
lipo -create -output "$OUT/craftpull-darwin-universal" \
  "$OUT/craftpull-darwin-amd64" "$OUT/craftpull-darwin-arm64"
lipo -info "$OUT/craftpull-darwin-universal"

echo "==> Пакет .app"
scripts/make-app-bundle.sh "$OUT/craftpull-darwin-universal" "$OUT/CraftPull.app"

echo "==> Готово:"
ls -lh "$OUT"
