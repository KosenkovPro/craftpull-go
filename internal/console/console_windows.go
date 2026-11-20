//go:build windows

package console

import (
	"os"
	"syscall"
)

// attachParentProcess — (DWORD)-1: присоединить консоль родительского процесса.
const attachParentProcess = ^uintptr(0)

var (
	kernel32          = syscall.NewLazyDLL("kernel32.dll")
	procAttachConsole = kernel32.NewProc("AttachConsole")
)

// Attach открывает консоль терминала, когда программа собрана как оконное
// приложение (-H windowsgui), и перенаправляет вывод в неё.
//
// Без этого сообщения --help и --version не видны: у оконного приложения
// нет ни консоли, ни стандартных потоков.
func Attach() {
	if ret, _, _ := procAttachConsole.Call(attachParentProcess); ret == 0 {
		// Консоль уже есть (обычная сборка) — ничего менять не нужно.
		return
	}
	if out, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0); err == nil {
		os.Stdout = out
	}
	if errOut, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0); err == nil {
		os.Stderr = errOut
	}
}
