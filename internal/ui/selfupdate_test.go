//go:build cgo

package ui

// Проверки раздела обновления программы: кнопки появляются в настройках
// и показывают найденную версию.

import (
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/KosenkovPro/craftpull-go/internal/app"
	"github.com/KosenkovPro/craftpull-go/internal/config"
	"github.com/KosenkovPro/craftpull-go/internal/i18n"
	"github.com/KosenkovPro/craftpull-go/internal/manager"
	"github.com/KosenkovPro/craftpull-go/internal/selfupdate"
)

// collectButtons собирает все кнопки в дереве объектов.
func collectButtons(obj fyne.CanvasObject) []*widget.Button {
	var out []*widget.Button
	var walk func(fyne.CanvasObject)
	walk = func(o fyne.CanvasObject) {
		switch v := o.(type) {
		case *widget.Button:
			out = append(out, v)
		case *fyne.Container:
			for _, child := range v.Objects {
				walk(child)
			}
		case *container.Scroll:
			walk(v.Content)
		}
	}
	walk(obj)
	return out
}

// findButton ищет кнопку по подписи.
func findButton(btns []*widget.Button, text string) *widget.Button {
	for _, b := range btns {
		if b.Text == text {
			return b
		}
	}
	return nil
}

// settingsUI собирает интерфейс на вкладке настроек.
func settingsUI(t *testing.T) *UI {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("CRAFTPULL_CONFIG_DIR", filepath.Join(dir, "config"))
	t.Setenv("CRAFTPULL_DATA_DIR", filepath.Join(dir, "data"))

	cfg := config.Config{
		RootDir:     filepath.Join(dir, "src"),
		MaxParallel: 2,
		Lang:        i18n.RU,
		ViewMode:    config.ViewGrid,
	}
	mgr, err := manager.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mgr.Close)

	st := app.New(config.ViewGrid, i18n.RU)
	u := NewUI(mgr, st)
	win := test.NewWindow(nil)
	win.SetContent(u.Build(win))
	win.Show()
	st.SetTab(app.TabSettings)
	u.Rebuild()
	return u
}

func TestSettingsUpdateSection(t *testing.T) {
	i18n.Set(i18n.RU)
	defer i18n.Set(i18n.EN)

	u := settingsUI(t)
	btns := collectButtons(u.content)

	check := findButton(btns, "Проверить обновления")
	if check == nil {
		t.Fatal("в настройках нет кнопки проверки обновлений")
	}
	if check.Disabled() {
		t.Error("кнопка проверки обновлений не должна быть заблокирована")
	}
	// Пока обновление не найдено, устанавливать нечего.
	apply := findButton(btns, "Обновлений нет")
	if apply == nil {
		t.Fatal("в настройках нет кнопки установки обновления")
	}
	if !apply.Disabled() {
		t.Error("кнопка установки должна быть заблокирована, пока обновление не найдено")
	}
}

func TestSettingsUpdateFound(t *testing.T) {
	i18n.Set(i18n.RU)
	defer i18n.Set(i18n.EN)

	u := settingsUI(t)
	u.SetSelfUpdate(&selfupdate.Update{Version: "v0.5.0"})
	u.Rebuild()

	btns := collectButtons(u.content)
	apply := findButton(btns, "Обновить до v0.5.0")
	if apply == nil {
		t.Fatalf("кнопка установки не показывает найденную версию: %v", buttonTexts(btns))
	}
	if apply.Disabled() {
		t.Error("кнопка установки должна быть активна, когда обновление найдено")
	}
}

// buttonTexts собирает подписи всех кнопок — для сообщений об ошибках.
func buttonTexts(btns []*widget.Button) []string {
	out := make([]string, 0, len(btns))
	for _, b := range btns {
		out = append(out, b.Text)
	}
	return out
}
