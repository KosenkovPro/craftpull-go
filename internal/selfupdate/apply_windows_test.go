//go:build windows

package selfupdate

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// helperEnv — переменная, по которой тест запускает самого себя помощником.
// Вспомогательный процесс заменяет файл программы и сразу завершается:
// так же поступает сама программа после установки обновления.
const helperEnv = "CRAFTPULL_APPLY_HELPER"

// TestMain разделяет обычный прогон тестов и запуск помощника.
func TestMain(m *testing.M) {
	if os.Getenv(helperEnv) == "1" {
		current := os.Getenv("CRAFTPULL_CURRENT")
		payload := os.Getenv("CRAFTPULL_PAYLOAD")
		if err := applyPlatform(current, payload); err != nil {
			os.Exit(3)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// helperSource — программа, которая оставляет след в файле рядом с собой.
func helperSource(marker string) string {
	return `package main

import (
	"os"
	"path/filepath"
)

func main() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(filepath.Dir(exe), "started.txt"), []byte("` + marker + `"), 0o644)
}
`
}

// buildHelper собирает вспомогательную программу.
func buildHelper(t *testing.T, dir, name, marker string) string {
	t.Helper()
	src := filepath.Join(dir, name+".go")
	if err := os.WriteFile(src, []byte(helperSource(marker)), 0o644); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, name+".exe")
	cmd := exec.Command("go", "build", "-o", exe, src)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("не удалось собрать помощника: %v\n%s", err, out)
	}
	return exe
}

// TestApplyWindowsReplacesAndRestarts проверяет главное место всей замены:
// файл программы подменяется новым, а новая копия действительно запускается.
//
// Замена делается вспомогательным скриптом уже после выхода программы,
// поэтому проверка идёт через отдельный процесс-помощник.
func TestApplyWindowsReplacesAndRestarts(t *testing.T) {
	dir := t.TempDir()
	oldExe := buildHelper(t, dir, "old", "old")
	newExe := buildHelper(t, dir, "new", "new")

	current := filepath.Join(dir, "craftpull.exe")
	if err := copyFile(oldExe, current); err != nil {
		t.Fatal(err)
	}

	// Помощник: заменяет файл и выходит, освобождая его.
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(self, "-test.run=^$")
	cmd.Env = append(os.Environ(),
		helperEnv+"=1",
		"CRAFTPULL_CURRENT="+current,
		"CRAFTPULL_PAYLOAD="+newExe,
	)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()

	// Ждём, пока скрипт дождётся конца помощника и запустит новую программу.
	marker := filepath.Join(dir, "started.txt")
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(marker)
		if err == nil && strings.TrimSpace(string(data)) == "new" {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	data, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("новая программа не запустилась: %v", err)
	}
	if got := strings.TrimSpace(string(data)); got != "new" {
		t.Fatalf("запустилась старая программа: %q", got)
	}

	// Файл программы теперь новый.
	want, err := os.ReadFile(newExe)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(current)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Error("файл программы не заменён новым")
	}
}

// TestHelperScriptRestoresOldProgram проверяет аварийный путь: если новый
// файл не удалось поставить на место, старая программа остаётся на месте —
// иначе приложение просто не запустится.
func TestHelperScriptRestoresOldProgram(t *testing.T) {
	dir := t.TempDir()
	current := filepath.Join(dir, "craftpull.exe")
	if err := os.WriteFile(current, []byte("old program"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Нового файла нет — сценарий не сможет его поставить на место.
	staged := filepath.Join(dir, "craftpull.exe.new.exe")
	script := filepath.Join(dir, "update.cmd")
	// Номер процесса, которого точно нет: ждать не придётся.
	content := helperScript(current, staged, 0x7FFFFFFF)
	if err := os.WriteFile(script, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("cmd", "/c", script).CombinedOutput()
	// Сценарий сообщает об аварии ненулевым кодом — это ожидаемо.
	if err == nil {
		t.Error("сценарий должен сообщить об аварии")
	}
	_ = out
	data, err := os.ReadFile(current)
	if err != nil {
		t.Fatalf("старая программа пропала: %v", err)
	}
	if string(data) != "old program" {
		t.Errorf("в файле программы %q, а должно остаться \"old program\"", data)
	}
}
