#!/usr/bin/env bash
# Собирает пакет CraftPull.app из готового бинарника macOS.
#
# Использование:
#   scripts/make-app-bundle.sh dist/craftpull-darwin-universal dist/CraftPull.app
set -euo pipefail

BIN="${1:?нужен путь к бинарнику}"
APP="${2:-dist/CraftPull.app}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

VERSION="$(grep -oE 'Version = "[^"]+"' internal/buildinfo/buildinfo.go | head -n1 | cut -d'"' -f2)"
MIN_MACOS="${MIN_MACOS:-12.0}"

rm -rf "$APP"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"
cp "$BIN" "$APP/Contents/MacOS/CraftPull"
chmod +x "$APP/Contents/MacOS/CraftPull"

# Иконка приложения (.icns собирается из PNG системной утилитой).
if [ -f craftpull-icon.png ] && command -v iconutil >/dev/null 2>&1; then
  ICONSET="$(mktemp -d)/CraftPull.iconset"
  mkdir -p "$ICONSET"
  for size in 16 32 128 256 512; do
    sips -z "$size" "$size" craftpull-icon.png --out "$ICONSET/icon_${size}x${size}.png" >/dev/null
    sips -z $((size * 2)) $((size * 2)) craftpull-icon.png --out "$ICONSET/icon_${size}x${size}@2x.png" >/dev/null
  done
  iconutil -c icns "$ICONSET" -o "$APP/Contents/Resources/CraftPull.icns"
  rm -rf "$(dirname "$ICONSET")"
fi

cat > "$APP/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleName</key><string>CraftPull</string>
  <key>CFBundleDisplayName</key><string>CraftPull</string>
  <key>CFBundleExecutable</key><string>CraftPull</string>
  <key>CFBundleIdentifier</key><string>io.github.kosenkovpro.craftpull</string>
  <key>CFBundleIconFile</key><string>CraftPull</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  <key>CFBundleShortVersionString</key><string>$VERSION</string>
  <key>CFBundleVersion</key><string>$VERSION</string>
  <key>LSMinimumSystemVersion</key><string>$MIN_MACOS</string>
  <key>NSHighResolutionCapable</key><true/>
  <key>NSHumanReadableCopyright</key><string>MIT / Apache-2.0</string>
</dict>
</plist>
PLIST

echo "Пакет готов: $APP"
