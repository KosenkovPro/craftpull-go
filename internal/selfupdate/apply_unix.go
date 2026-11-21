//go:build !windows

package selfupdate

import (
	"os"
	"os/exec"
	"path/filepath"
)

// applyPlatform заменяет файл программы и запускает новую версию.
//
// На macOS и Linux запускаемый файл можно переименовать: сначала текущий
// уезжает в .old, потом на его место встаёт новый. Если программа запущена
// из пакета .app, меняется файл внутри пакета — сам пакет остаётся целым.
func applyPlatform(current, payload string) error {
	dir := filepath.Dir(current)
	base := filepath.Base(current)
	newPath := filepath.Join(dir, "."+base+".new")
	oldPath := filepath.Join(dir, "."+base+".old")
	_ = os.Remove(newPath)
	_ = os.Remove(oldPath)

	if err := copyFile(payload, newPath); err != nil {
		return err
	}
	if err := os.Chmod(newPath, 0o755); err != nil {
		return err
	}
	if err := os.Rename(current, oldPath); err != nil {
		return err
	}
	if err := os.Rename(newPath, current); err != nil {
		// Возвращаем как было — программа не должна остаться без файла.
		_ = os.Rename(oldPath, current)
		return err
	}
	_ = os.Remove(oldPath)

	cmd := exec.Command(current, os.Args[1:]...)
	cmd.Dir = dir
	return cmd.Start()
}
