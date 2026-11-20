//go:build windows

package shortcut

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/KosenkovPro/craftpull-go/internal/config"
)

// desktopPath ищет папку рабочего стола (может быть перенаправлена в OneDrive).
func desktopPath() (string, error) {
	if d := os.Getenv("USERPROFILE"); d != "" {
		p := filepath.Join(d, "Desktop")
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	out, err := exec.Command("powershell", "-NoProfile", "-Command",
		"[Environment]::GetFolderPath('Desktop')").Output()
	if err == nil {
		p := strings.TrimSpace(string(out))
		if p != "" {
			return p, nil
		}
	}
	return "", fmt.Errorf("не удалось найти рабочий стол")
}

// createShortcut создаёт ярлык CraftPull.lnk на рабочем столе.
func createShortcut(target string) (string, error) {
	desktop, err := desktopPath()
	if err != nil {
		return "", err
	}
	lnk := filepath.Join(desktop, "CraftPull.lnk")
	icon := filepath.Join(InstallPath(), "craftpull.ico")

	script := fmt.Sprintf(`
$ErrorActionPreference = 'Stop'
$s = (New-Object -ComObject WScript.Shell).CreateShortcut('%s')
$s.TargetPath = '%s'
$s.WorkingDirectory = '%s'
$s.Description = 'CraftPull — release manager'
%s
$s.Save()
`, lnk, target, filepath.Dir(target), iconOption(icon))

	cmd := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("%s: %s", err, strings.TrimSpace(string(out)))
	}
	return lnk, nil
}

func iconOption(icon string) string {
	if _, err := os.Stat(icon); err == nil {
		return fmt.Sprintf("$s.IconLocation = '%s'", icon)
	}
	return ""
}

func dataDir() string { return config.DataDir() }
func logPath() string { return config.LogPath() }

// openPath открывает путь в проводнике.
func openPath(path string) error {
	return exec.Command("explorer", path).Start()
}
