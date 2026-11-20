package app

import (
	"runtime"
	"testing"

	"github.com/KosenkovPro/craftpull-go/internal/config"
	"github.com/KosenkovPro/craftpull-go/internal/model"
)

// TestPreselect проверяет, что файл под текущую ОС отмечается один раз.
func TestPreselect(t *testing.T) {
	st := New(config.ViewGrid, 0)
	rel := &model.ReleaseInfo{
		Tag: "v1.0.0",
		Assets: []model.AssetInfo{
			{Name: "app-1.0.0-windows-x64.zip", Size: 100},
			{Name: "app-1.0.0-linux-x86_64.AppImage", Size: 200},
		},
	}

	// Отмечается файл под текущую ОС: на Windows — архив, на Linux и
	// macOS — AppImage.
	expected := "app-1.0.0-linux-x86_64.AppImage"
	if runtime.GOOS == "windows" {
		expected = "app-1.0.0-windows-x64.zip"
	}

	st.Preselect("owner/app", rel)
	if !st.IsSelected(rel.Tag, expected) {
		t.Fatalf("файл под текущую ОС (%s) не отмечен", expected)
	}

	count, _, _ := st.SelectionSummary([]model.ReleaseInfo{*rel})
	if count != 1 {
		t.Fatalf("отмечено файлов: %d, ожидался 1", count)
	}

	// Повторный вызов не должен отмечать файл заново после снятия отметки.
	st.ClearSelection()
	st.Preselect("owner/app", rel)
	if st.IsSelected(rel.Tag, expected) {
		t.Fatal("отметка вернулась после снятия")
	}
}
