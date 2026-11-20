//go:build !windows

package console

// Attach — ничего не делает: в консольных системах вывод и так доступен.
func Attach() {}
