//go:build !windows && !darwin

package shortcut

import (
	"os/exec"

	"github.com/KosenkovPro/craftpull-go/internal/config"
)

// createShortcut на Linux создаёт ссылку в ~/bin.
func createShortcut(target string) (string, error) {
	return target, nil
}

func dataDir() string { return config.DataDir() }
func logPath() string { return config.LogPath() }

// openPath открывает путь в файловом менеджере.
func openPath(path string) error {
	return exec.Command("xdg-open", path).Start()
}
