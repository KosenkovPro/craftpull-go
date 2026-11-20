//go:build darwin

package shortcut

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/KosenkovPro/craftpull-go/internal/buildinfo"
	"github.com/KosenkovPro/craftpull-go/internal/config"
)

// createShortcut создаёт пакет CraftPull.app в ~/Applications.
func createShortcut(target string) (string, error) {
	bundle := filepath.Join(InstallPath(), "CraftPull.app")
	macOSDir := filepath.Join(bundle, "Contents", "MacOS")
	resources := filepath.Join(bundle, "Contents", "Resources")
	for _, dir := range []string{macOSDir, resources} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
	}

	// Бинарник внутри пакета.
	data, err := os.ReadFile(target)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(macOSDir, "craftpull"), data, 0o755); err != nil {
		return "", err
	}

	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleName</key><string>CraftPull</string>
  <key>CFBundleDisplayName</key><string>CraftPull</string>
  <key>CFBundleIdentifier</key><string>store.chekak.craftpull</string>
  <key>CFBundleExecutable</key><string>craftpull</string>
  <key>CFBundleIconFile</key><string>craftpull.icns</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  <key>CFBundleShortVersionString</key><string>` + buildinfo.Version + `</string>
  <key>CFBundleVersion</key><string>` + buildinfo.Version + `</string>
  <key>LSMinimumSystemVersion</key><string>12.0</string>
  <key>NSHighResolutionCapable</key><true/>
</dict>
</plist>
`
	if err := os.WriteFile(filepath.Join(bundle, "Contents", "Info.plist"), []byte(plist), 0o644); err != nil {
		return "", err
	}
	return bundle, nil
}

func dataDir() string { return config.DataDir() }
func logPath() string { return config.LogPath() }

// openPath открывает путь в программе по умолчанию.
func openPath(path string) error {
	return exec.Command("open", path).Start()
}

// OpenApp открывает папку с данными через Finder.
var _ = fmt.Sprintf
