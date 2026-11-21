//go:build windows

package selfupdate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
)

// applyPlatform подменяет файл программы через вспомогательный сценарий.
//
// Запущенный .exe нельзя перезаписать, поэтому маленький .cmd ждёт завершения
// программы, переносит новый файл на место и запускает его.
func applyPlatform(current, payload string) error {
	dir := filepath.Dir(current)
	staged := filepath.Join(dir, filepath.Base(current)+".new.exe")
	_ = os.Remove(staged)
	if err := copyFile(payload, staged); err != nil {
		return err
	}

	script := filepath.Join(os.TempDir(), fmt.Sprintf("craftpull-update-%d.cmd", os.Getpid()))
	pid := strconv.Itoa(os.Getpid())
	// Пауза сделана через ping: команда timeout не работает, когда ввод
	// перенаправлен, а программа запущена без окна.
	content := fmt.Sprintf(`@echo off
:wait
tasklist /FI "PID eq %[1]s" /NH 2>nul | find "%[1]s" >nul
if not errorlevel 1 (ping -n 2 127.0.0.1 >nul & goto wait)
move /Y "%[2]s" "%[2]s.old" >nul 2>nul
move /Y "%[3]s" "%[2]s" >nul 2>nul
start "" "%[2]s"
del "%[2]s.old" >nul 2>nul
del "%%~f0" >nul 2>nul
`, pid, current, staged)
	if err := os.WriteFile(script, []byte(content), 0o644); err != nil {
		return err
	}

	cmd := exec.Command("cmd", "/c", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	return nil
}
