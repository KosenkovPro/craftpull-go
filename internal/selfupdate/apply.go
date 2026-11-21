package selfupdate

// Apply подменяет текущий файл программы новым и запускает его заново.
//
// После успешного вызова программа должна завершиться: на Windows старый
// файл ещё занят запущенным процессом, на macOS и Linux — уже заменён.
//
// Само подменение делает applyPlatform — она своя для каждой системы.
func Apply(current, payload string) error {
	return applyPlatform(current, payload)
}
