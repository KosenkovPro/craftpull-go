//go:build cgo

package ui

// Проверка отрисовки всех экранов интерфейса.
//
// Тест ловит паники при сборке экранов и заодно обновляет иллюстрации для
// README: если задана переменная окружения CRAFTPULL_SHOTS_DIR, снимки
// сохраняются в указанную папку.
//
//	go test ./internal/ui -run TestScreens -count=1
//	$env:CRAFTPULL_SHOTS_DIR = "docs/screenshots"
//	go test ./internal/ui -run TestScreens -count=1

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/KosenkovPro/craftpull-go/internal/app"
	"github.com/KosenkovPro/craftpull-go/internal/config"
	"github.com/KosenkovPro/craftpull-go/internal/i18n"
	"github.com/KosenkovPro/craftpull-go/internal/manager"
	"github.com/KosenkovPro/craftpull-go/internal/model"
)

// shotsEnv — папка для снимков экранов.
const shotsEnv = "CRAFTPULL_SHOTS_DIR"

// shotRepos — репозитории для снимков: те же, что и в рабочей копии,
// с описаниями из карточек GitHub.
func shotRepos() []config.RepoConfig {
	list := []struct{ slug, desc string }{
		{"storytold/pdfcraft", "PdfCraft: An open-source PDF reader and editor written from scratch in pure Rust"},
		{"storytold/wordcraft", "WordCraft: an open-source, clean-room word processor in pure Rust"},
		{"storytold/gridcraft", "GridCraft: an open-source, clean-room spreadsheet in pure Rust. By ArtCraft."},
		{"storytold/deckcraft", "DeckCraft: presentations and slide shows; an open-source, clean-room presentation app in pure Rust."},
		{"storytold/photocraft", "PhotoCraft: An open-source photo editor written from scratch in pure Rust"},
		{"storytold/vectorcraft", "VectorCraft: An open-source vector art tool written from scratch in pure Rust"},
		{"storytold/figcraft", "FigCraft: interface design and prototyping; an open-source, clean-room reimplementation of collaborative design workflows in pure Rust"},
		{"storytold/designcraft", "DesignCraft: An open-source page and layout design editor written from scratch in pure Rust"},
		{"storytold/cadcraft", "CADCraft: computer-aided design and drafting — an open-source, clean-room CAD application in pure Rust"},
	}
	out := make([]config.RepoConfig, 0, len(list))
	for _, r := range list {
		rc := config.NewRepoConfig(r.slug)
		rc.Enabled = true
		rc.Description = r.desc
		out = append(out, rc)
	}
	return out
}

// TestScreens проверяет, что все экраны интерфейса собираются и рисуются.
func TestScreens(t *testing.T) {
	// Путь к локальному кэшу релизов: на снимках нужны настоящие версии,
	// поэтому кэш копируется в папку теста, а сеть не требуется.
	realCache := config.CachePath()

	dir := t.TempDir()
	t.Setenv("CRAFTPULL_CONFIG_DIR", filepath.Join(dir, "config"))
	t.Setenv("CRAFTPULL_DATA_DIR", filepath.Join(dir, "data"))

	i18n.Set(i18n.RU)
	defer i18n.Set(i18n.EN)

	cfg := config.Config{
		RootDir:           filepath.Join(dir, "src"),
		MaxParallel:       4,
		Lang:              i18n.RU,
		ViewMode:          config.ViewGrid,
		DeleteOldVersions: true,
		VerifySHA256:      true,
	}
	cfg.Repos = shotRepos()

	if data, err := os.ReadFile(realCache); err == nil {
		if err := os.MkdirAll(filepath.Dir(config.CachePath()), 0o755); err == nil {
			_ = os.WriteFile(config.CachePath(), data, 0o644)
		}
	}

	mgr, err := manager.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Close()

	// Журнал на снимке должен быть живым, а не пустым.
	mgr.Log(model.LogInfo, "Проверка репозиториев: storytold/pdfcraft")
	mgr.Log(model.LogOk, "Найден новый релиз v0.23.1")
	mgr.Log(model.LogWarn, "Лимит GitHub API исчерпан, читаю страницу релиза")
	mgr.Log(model.LogError, "Не удалось скачать pdfcraft-0.23.1-windows-x64.zip")

	st := app.New(config.ViewGrid, i18n.RU)
	u := NewUI(mgr, st)
	win := test.NewWindow(nil)
	win.Resize(fyne.NewSize(1440, 900))
	win.SetContent(u.Build(win))
	win.Show()

	shots := strings.TrimSpace(os.Getenv(shotsEnv))
	shoot := func(name string) {
		t.Helper()
		captureScreen(t, win, shots, name)
	}

	// Сетка: карточки репозиториев.
	shoot("01-repositories-grid.png")

	// Список: подробные карточки.
	st.SetView(config.ViewList)
	u.Rebuild()
	shoot("02-repositories-list.png")

	// Режим без картинок.
	st.SetView(config.ViewLight)
	u.Rebuild()
	shoot("03-repositories-light.png")

	// Панель релизов с выбором файлов.
	st.SetView(config.ViewGrid)
	st.OpenReleases("storytold/pdfcraft")
	u.Rebuild()
	shoot("04-releases.png")
	st.CloseReleases()

	// Настройки.
	st.SetTab(app.TabSettings)
	u.Rebuild()
	shoot("05-settings.png")

	// Журнал.
	st.SetTab(app.TabLogs)
	u.Rebuild()
	shoot("06-logs.png")
}

// captureScreen сохраняет снимок окна, если задана папка для снимков.
func captureScreen(t *testing.T, win fyne.Window, dir, name string) {
	t.Helper()
	// Даём Fyne перерисовать окно: снимок должен показать свежую раскладку.
	time.Sleep(200 * time.Millisecond)
	img := win.Canvas().Capture()
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	t.Logf("снимок сохранён: %s", path)
}
