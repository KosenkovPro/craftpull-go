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
	list := []struct{ slug, desc, purpose, cat string }{
		{"storytold/pdfcraft", "PdfCraft: An open-source PDF reader and editor written from scratch in pure Rust", "PDF-документы", config.CatOwn},
		{"storytold/wordcraft", "WordCraft: an open-source, clean-room word processor in pure Rust", "Текстовый редактор", config.CatOwn},
		{"storytold/gridcraft", "GridCraft: an open-source, clean-room spreadsheet in pure Rust. By ArtCraft.", "Таблицы", config.CatOwn},
		{"storytold/vectorcraft", "VectorCraft: An open-source vector art tool written from scratch in pure Rust", "Векторная графика", config.CatOwn},
		{"storytold/filmcraft", "FilmCraft: a video editor written from scratch in pure Rust", "Монтаж видео", config.CatOwn},
		{"flameshot-org/flameshot", "Powerful yet simple to use screenshot software", "Скриншоты и аннотации", config.CatCapture},
		{"obsproject/obs-studio", "OBS Studio — live streaming and screen recording", "Запись и стриминг", config.CatCapture},
		{"mifi/lossless-cut", "LosslessCut — cut and join video without re-encoding", "Нарезка видео", config.CatMedia},
		{"audacity/audacity", "Audacity — record and edit audio", "Запись и монтаж звука", config.CatMedia},
		{"mpv-player/mpv", "A media player plays most formats, codecs, streams and devices", "Всеядный медиаплеер", config.CatMedia},
		{"darktable-org/darktable", "darktable is a virtual darkroom for raw photographs", "Растровый фоторедактор", config.CatGraphics},
		{"FreeCAD/FreeCAD", "FreeCAD is a free and open-source parametric 3D CAD modeler", "3D-моделирование и CAD", config.CatGraphics},
		{"dundee/gdu", "Fast disk usage analyzer", "Анализ занятого места", config.CatSystem},
		{"iterate-ch/cyberduck", "Cyberduck is a libre FTP, SFTP, WebDAV and S3 client", "FTP / SFTP клиент", config.CatSystem},
		{"rustdesk/rustdesk", "An open-source remote desktop alternative to TeamViewer and AnyDesk", "Удалённый рабочий стол", config.CatSystem},
		{"keepassxreboot/keepassxc", "The official fork of KeePass, a password manager", "Менеджер паролей", config.CatSecurity},
		{"kiwix/kiwix-android", "Kiwix is an offline reader for Wikipedia and other content", "Википедия (Android)", config.CatSecurity},
		{"jellyfin/jellyfin", "Free Software Media System that is an alternative to Plex", "Домашний кинотеатр", config.CatServer},
		{"Kareadita/Kavita", "Kavita is a digital library server", "Сервер книг и PDF", config.CatServer},
		{"advplyr/audiobookshelf", "Self-hosted audiobook and podcast server", "Сервер аудиокниг", config.CatServer},
	}
	out := make([]config.RepoConfig, 0, len(list))
	for _, r := range list {
		rc := config.NewRepoConfig(r.slug)
		rc.Enabled = true
		rc.Description = r.desc
		rc.Purpose = r.purpose
		rc.Category = r.cat
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

	// Раскрытая карточка репозитория — как в Windows-версии.
	st.SetTab(app.TabRepos)
	st.SetHero("storytold/pdfcraft")
	u.Rebuild()
	shoot("07-repositories-hero.png")
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
