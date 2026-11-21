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

echo "==> Пакеты .app"
# Отдельный пакет под каждую архитектуру: кому не нужен universal.
scripts/make-app-bundle.sh "$OUT/craftpull-darwin-amd64" "$OUT/amd64/CraftPull.app"
scripts/make-app-bundle.sh "$OUT/craftpull-darwin-arm64" "$OUT/arm64/CraftPull.app"
scripts/make-app-bundle.sh "$OUT/craftpull-darwin-universal" "$OUT/CraftPull.app"

echo "==> Архивы пакетов"
( cd "$OUT/amd64" && zip -qry ../craftpull-macos-amd64.zip CraftPull.app )
( cd "$OUT/arm64" && zip -qry ../craftpull-macos-arm64.zip CraftPull.app )
( cd "$OUT" && zip -qry craftpull-macos-universal.zip CraftPull.app )

echo "==> Образ диска"
chmod +x scripts/make-dmg.sh scripts/make-pkg.sh
scripts/make-dmg.sh "$OUT/CraftPull.app" "$OUT/craftpull-macos-universal.dmg"

echo "==> Установщик .pkg"
scripts/make-pkg.sh "$OUT/CraftPull.app" "$OUT/craftpull-macos-universal.pkg"

echo "==> Готово:"
ls -lh "$OUT"
