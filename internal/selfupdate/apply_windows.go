//go:build windows

package selfupdate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// scriptPrefix — начало имени вспомогательного сценария замены.
const scriptPrefix = "craftpull-update-"

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

	// Сценарии прошлых обновлений больше не нужны — убираем их.
	cleanOldScripts()

	script := filepath.Join(os.TempDir(), fmt.Sprintf("%s%d.cmd", scriptPrefix, os.Getpid()))
	if err := os.WriteFile(script, []byte(helperScript(current, staged, os.Getpid())), 0o644); err != nil {
		return err
	}

	cmd := exec.Command("cmd", "/c", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	return nil
}

// helperScript собирает вспомогательный сценарий замены программы.
//
// Сценарий ждёт, пока закончится процесс с указанным номером, убирает старую
// программу в сторону, ставит на её место новую и запускает.
//
// Если новый файл не встал на место — старая программа возвращается обратно:
// без неё приложение просто не запустится.
//
// Сценарий сам себя не удаляет: файл ещё читается, и удаление на ходу обрывает
// выполнение. Убором старых сценариев занимается cleanOldScripts.
func helperScript(current, staged string, pid int) string {
	return fmt.Sprintf(`@echo off
:wait
tasklist /FI "PID eq %[1]s" /NH 2>nul | find "%[1]s" >nul
if not errorlevel 1 (ping -n 2 127.0.0.1 >nul & goto wait)
move /Y "%[2]s" "%[2]s.old" >nul 2>nul
if exist "%[2]s" goto fail
move /Y "%[3]s" "%[2]s" >nul 2>nul
if not exist "%[2]s" goto fail
start "" "%[2]s"
del "%[2]s.old" >nul 2>nul
exit /b 0
:fail
if exist "%[2]s.old" move /Y "%[2]s.old" "%[2]s" >nul 2>nul
exit /b 1
`, strconv.Itoa(pid), current, staged)
}

// cleanOldScripts убирает сценарии прошлых обновлений.
//
// Сценарий замены остаётся на диске до следующего обновления: удалять его
// изнутри нельзя — файл ещё читается. Старые сценарии убираются здесь.
func cleanOldScripts() {
	dir := os.TempDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-time.Hour)
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, scriptPrefix) || !strings.HasSuffix(name, ".cmd") {
			continue
		}
		info, err := e.Info()
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		_ = os.Remove(filepath.Join(dir, name))
	}
}
