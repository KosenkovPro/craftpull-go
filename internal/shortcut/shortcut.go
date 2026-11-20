// Package shortcut — установка программы: ярлык на рабочем столе (Windows)
// или пакет .app в ~/Applications (macOS).
package shortcut

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// InstallPath — куда копируется программа.
func InstallPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	switch runtime.GOOS {
	case "windows":
		local := os.Getenv("LOCALAPPDATA")
		if local == "" {
			local = home
		}
		return filepath.Join(local, "Programs", "CraftPull")
	case "darwin":
		return filepath.Join(home, "Applications", "CraftPull")
	default:
		return filepath.Join(home, ".local", "bin")
	}
}

// Install копирует программу в папку установки и создаёт ярлык.
// Возвращает путь к созданному ярлыку.
func Install() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		exe, _ = os.Executable()
	}
	dir := InstallPath()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	target := filepath.Join(dir, "craftpull")
	if runtime.GOOS == "windows" {
		target += ".exe"
	}
	data, err := os.ReadFile(exe)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(target, data, 0o755); err != nil {
		return "", err
	}
	return createShortcut(target)
}

// OpenDataDir открывает папку с данными программы.
func OpenDataDir() error {
	dir := dataDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return openPath(dir)
}

// OpenLogFile открывает файл журнала.
func OpenLogFile() error {
	path := logPath()
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("файла журнала пока нет")
	}
	return openPath(path)
}
