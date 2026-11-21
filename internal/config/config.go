// Package config — настройки приложения, список репозиториев, формат JSON для обмена списком.
//
// Формат файла совместим с Rust-версией CraftPull: те же имена полей и те же
// значения перечислений, поэтому конфиги переносятся между версиями.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/KosenkovPro/craftpull-go/internal/i18n"
)

// MaxParallel — сколько файлов можно качать одновременно (максимум).
const MaxParallel = 16

// DefaultRepos — репозитории, которые добавляются при первом запуске.
//
// Список разбит по назначению программ: скриншоты и запись экрана, видео и
// аудио, графика, системные утилиты, оффлайн-базы и self-hosted серверы.
// В категориях, где у программы нет бинарных релизов на GitHub, приложение
// скачивает архив с исходниками.
var DefaultRepos = []string{
	// Свои программы.
	"storytold/pdfcraft",
	"storytold/wordcraft",
	"storytold/gridcraft",
	"storytold/deckcraft",
	"storytold/photocraft",
	"storytold/vectorcraft",
	"storytold/figcraft",
	"storytold/designcraft",
	"storytold/cadcraft",
	"storytold/texturecraft",
	"storytold/materialcraft",
	"storytold/filmcraft",
	"storytold/lightcraft",
	"storytold/effectcraft",
	"storytold/soundcraft",
	"storytold/artcraft",
	"storytold/craft-anything",
	"storytold/craft-libs",
	"storytold/craft-launcher",

	// Скриншоты и запись экрана.
	"flameshot-org/flameshot",
	"obsproject/obs-studio",
	"NickeManarin/ScreenToGif",

	// Видео, аудио и конвертация.
	"mifi/lossless-cut",
	"HandBrake/HandBrake",
	"audacity/audacity",
	"mpv-player/mpv",
	"jeffvli/feishin",
	"SV-stark/FileConverter-rs",

	// Графика и 3D.
	"d2phap/ImageGlass",
	"darktable-org/darktable",
	"FreeCAD/FreeCAD",

	// Система, файлы и сеть.
	"microsoft/PowerToys",
	"dundee/gdu",
	"iterate-ch/cyberduck",
	"qbittorrent/qBittorrent",
	"pbatard/rufus",
	"rustdesk/rustdesk",

	// Безопасность, карты и оффлайн-базы.
	"keepassxreboot/keepassxc",
	"organicmaps/organicmaps",
	"kiwix/kiwix-desktop",
	"kiwix/kiwix-android",

	// Self-hosted серверы.
	"passbolt/passbolt_api",
	"immich-app/immich",
	"owncloud/ocis",
	"nextcloud/server",
	"Freika/dawarich",
	"jellyfin/jellyfin",
	"kieraneglin/pinchflat",
	"navidrome/navidrome",
	"advplyr/audiobookshelf",
	"Kareadita/Kavita",
	"owncast/owncast",
	"kiwix/kiwix-tools",
	"VERT-sh/VERT",
	"VERT-sh/vertd",
}

// DefaultPurposes — короткое назначение программы.
//
// Это пользовательские данные, а не переводы интерфейса: они хранятся
// в конфиге, правятся вручную и переживают экспорт и импорт списка.
// Тексты короткие, чтобы целиком помещаться в плитке сетки.
var DefaultPurposes = map[string]string{
	"storytold/pdfcraft":       "PDF-документы",
	"storytold/wordcraft":      "Текстовый редактор",
	"storytold/gridcraft":      "Таблицы",
	"storytold/deckcraft":      "Презентации",
	"storytold/photocraft":     "Работа с фото",
	"storytold/vectorcraft":    "Векторная графика",
	"storytold/figcraft":       "Прототипы",
	"storytold/designcraft":    "Дизайн и вёрстка",
	"storytold/cadcraft":       "Чертежи и CAD",
	"storytold/texturecraft":   "Текстуры",
	"storytold/materialcraft":  "Материалы",
	"storytold/filmcraft":      "Монтаж видео",
	"storytold/lightcraft":     "Освещение",
	"storytold/effectcraft":    "Эффекты",
	"storytold/soundcraft":     "Звук и музыка",
	"storytold/artcraft":       "Живопись",
	"storytold/craft-anything": "Всё сразу",
	"storytold/craft-libs":     "Библиотеки",
	"storytold/craft-launcher": "Запуск программ",

	// Скриншоты и запись экрана.
	"flameshot-org/flameshot":  "Скриншоты и аннотации",
	"obsproject/obs-studio":    "Запись и стриминг",
	"NickeManarin/ScreenToGif": "Запись в GIF/MP4",

	// Видео, аудио и конвертация.
	"mifi/lossless-cut":         "Нарезка видео",
	"HandBrake/HandBrake":       "Сжатие видео",
	"audacity/audacity":         "Запись и монтаж звука",
	"mpv-player/mpv":            "Всеядный медиаплеер",
	"jeffvli/feishin":           "Плеер для сервера",
	"SV-stark/FileConverter-rs": "Конвертер файлов",

	// Графика и 3D.
	"d2phap/ImageGlass":       "Просмотрщик фото",
	"darktable-org/darktable": "Растровый фоторедактор",
	"FreeCAD/FreeCAD":         "3D-моделирование и CAD",

	// Система, файлы и сеть.
	"microsoft/PowerToys":     "Утилиты Windows",
	"dundee/gdu":              "Анализ занятого места",
	"iterate-ch/cyberduck":    "FTP / SFTP клиент",
	"qbittorrent/qBittorrent": "Торрент-клиент",
	"pbatard/rufus":           "Запись флешек USB",
	"rustdesk/rustdesk":       "Удалённый рабочий стол",

	// Безопасность, карты и оффлайн-базы.
	"keepassxreboot/keepassxc": "Менеджер паролей",
	"organicmaps/organicmaps":  "Карты и навигатор",
	"kiwix/kiwix-desktop":      "Оффлайн Википедия",
	"kiwix/kiwix-android":      "Википедия (Android)",

	// Self-hosted серверы.
	"passbolt/passbolt_api":  "Сервер паролей и 2FA",
	"immich-app/immich":      "Аналог Google Фото",
	"owncloud/ocis":          "Файловое облако",
	"nextcloud/server":       "Календари и контакты",
	"Freika/dawarich":        "История поездок",
	"jellyfin/jellyfin":      "Домашний кинотеатр",
	"kieraneglin/pinchflat":  "Архив YouTube-каналов",
	"navidrome/navidrome":    "Музыкальный сервер",
	"advplyr/audiobookshelf": "Сервер аудиокниг",
	"Kareadita/Kavita":       "Сервер книг и PDF",
	"owncast/owncast":        "Сервер прямых эфиров",
	"kiwix/kiwix-tools":      "Веб-сервер Википедии",
	"VERT-sh/VERT":           "Веб-конвертер файлов",
	"VERT-sh/vertd":          "Демон веб-конвертера",
}

// DefaultCategories — категория программы для группировки списка.
//
// Ключи категорий переводятся интерфейсом (cat.*), поэтому в конфиге лежит
// только ключ. Своё значение пользователь может поменять вручную.
var DefaultCategories = map[string]string{
	"storytold/pdfcraft":       CatOwn,
	"storytold/wordcraft":      CatOwn,
	"storytold/gridcraft":      CatOwn,
	"storytold/deckcraft":      CatOwn,
	"storytold/photocraft":     CatOwn,
	"storytold/vectorcraft":    CatOwn,
	"storytold/figcraft":       CatOwn,
	"storytold/designcraft":    CatOwn,
	"storytold/cadcraft":       CatOwn,
	"storytold/texturecraft":   CatOwn,
	"storytold/materialcraft":  CatOwn,
	"storytold/filmcraft":      CatOwn,
	"storytold/lightcraft":     CatOwn,
	"storytold/effectcraft":    CatOwn,
	"storytold/soundcraft":     CatOwn,
	"storytold/artcraft":       CatOwn,
	"storytold/craft-anything": CatOwn,
	"storytold/craft-libs":     CatOwn,
	"storytold/craft-launcher": CatOwn,

	// Скриншоты и запись экрана.
	"flameshot-org/flameshot":  CatCapture,
	"obsproject/obs-studio":    CatCapture,
	"NickeManarin/ScreenToGif": CatCapture,

	// Видео, аудио и конвертация.
	"mifi/lossless-cut":         CatMedia,
	"HandBrake/HandBrake":       CatMedia,
	"audacity/audacity":         CatMedia,
	"mpv-player/mpv":            CatMedia,
	"jeffvli/feishin":           CatMedia,
	"SV-stark/FileConverter-rs": CatMedia,

	// Графика и 3D.
	"d2phap/ImageGlass":       CatGraphics,
	"darktable-org/darktable": CatGraphics,
	"FreeCAD/FreeCAD":         CatGraphics,

	// Система, файлы и сеть.
	"microsoft/PowerToys":     CatSystem,
	"dundee/gdu":              CatSystem,
	"iterate-ch/cyberduck":    CatSystem,
	"qbittorrent/qBittorrent": CatSystem,
	"pbatard/rufus":           CatSystem,
	"rustdesk/rustdesk":       CatSystem,

	// Безопасность, карты и оффлайн-базы.
	"keepassxreboot/keepassxc": CatSecurity,
	"organicmaps/organicmaps":  CatSecurity,
	"kiwix/kiwix-desktop":      CatSecurity,
	"kiwix/kiwix-android":      CatSecurity,

	// Self-hosted серверы.
	"passbolt/passbolt_api":  CatServer,
	"immich-app/immich":      CatServer,
	"owncloud/ocis":          CatServer,
	"nextcloud/server":       CatServer,
	"Freika/dawarich":        CatServer,
	"jellyfin/jellyfin":      CatServer,
	"kieraneglin/pinchflat":  CatServer,
	"navidrome/navidrome":    CatServer,
	"advplyr/audiobookshelf": CatServer,
	"Kareadita/Kavita":       CatServer,
	"owncast/owncast":        CatServer,
	"kiwix/kiwix-tools":      CatServer,
	"VERT-sh/VERT":           CatServer,
	"VERT-sh/vertd":          CatServer,
}

// Ключи категорий: их названия переводятся интерфейсом.
const (
	CatOwn      = "own"
	CatCapture  = "capture"
	CatMedia    = "media"
	CatGraphics = "graphics"
	CatSystem   = "system"
	CatSecurity = "security"
	CatServer   = "server"
	CatOther    = "other"
)

// CategoryOrder — порядок категорий в списке.
var CategoryOrder = []string{
	CatOwn, CatCapture, CatMedia, CatGraphics, CatSystem, CatSecurity, CatServer, CatOther,
}

// CategoryTitle — название категории для заголовка группы.
func CategoryTitle(key string) string {
	switch key {
	case CatOwn:
		return i18n.T("cat.own")
	case CatCapture:
		return i18n.T("cat.capture")
	case CatMedia:
		return i18n.T("cat.media")
	case CatGraphics:
		return i18n.T("cat.graphics")
	case CatSystem:
		return i18n.T("cat.system")
	case CatSecurity:
		return i18n.T("cat.security")
	case CatServer:
		return i18n.T("cat.server")
	}
	return i18n.T("cat.other")
}

// AssetKind — какой файл релиза скачивать.
//
// Значение выбирается из реальных файлов релиза: приложение определяет вид
// файла по имени и подставляет нужный под текущую ОС.
type AssetKind string

const (
	// AssetAuto — автоматически: файл под текущую ОС (по умолчанию).
	AssetAuto AssetKind = "Auto"
	// AssetWindowsPortable — Windows: portable-архив (zip, без установщика).
	AssetWindowsPortable AssetKind = "WindowsPortable"
	// AssetWindowsMsi — Windows: MSI-установщик.
	AssetWindowsMsi AssetKind = "WindowsMsi"
	// AssetWindowsSetup — Windows: установщик setup.exe.
	AssetWindowsSetup AssetKind = "WindowsSetup"
	// AssetLinuxAppImage — Ubuntu: AppImage.
	AssetLinuxAppImage AssetKind = "LinuxAppImage"
	// AssetLinuxTarGz — Ubuntu: tar.gz.
	AssetLinuxTarGz AssetKind = "LinuxTarGz"
	// AssetLinuxDeb — Ubuntu: deb.
	AssetLinuxDeb AssetKind = "LinuxDeb"
	// AssetLinuxRpm — Ubuntu: rpm.
	AssetLinuxRpm AssetKind = "LinuxRpm"
	// AssetSource — только архив с исходниками.
	AssetSource AssetKind = "Source"
	// AssetAll — все файлы релиза.
	AssetAll AssetKind = "All"
)

// UnmarshalJSON принимает старое значение "Portable" как синоним "Auto".
func (k *AssetKind) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch AssetKind(s) {
	case "Portable":
		*k = AssetAuto
		return nil
	}
	switch AssetKind(s) {
	case AssetAuto, AssetWindowsPortable, AssetWindowsMsi, AssetWindowsSetup,
		AssetLinuxAppImage, AssetLinuxTarGz, AssetLinuxDeb, AssetLinuxRpm,
		AssetSource, AssetAll:
		*k = AssetKind(s)
		return nil
	}
	// Неизвестное значение — считаем авто-выбором, программа не должна падать.
	*k = AssetAuto
	return nil
}

// Label возвращает подпись с учётом языка интерфейса.
func (k AssetKind) Label() string {
	switch k {
	case AssetAuto:
		return i18n.T("filter.auto")
	case AssetWindowsPortable:
		return i18n.T("filter.windows_portable")
	case AssetWindowsMsi:
		return i18n.T("filter.windows_msi")
	case AssetWindowsSetup:
		return i18n.T("filter.windows_setup")
	case AssetLinuxAppImage:
		return i18n.T("filter.linux_appimage")
	case AssetLinuxTarGz:
		return i18n.T("filter.linux_targz")
	case AssetLinuxDeb:
		return i18n.T("filter.linux_deb")
	case AssetLinuxRpm:
		return i18n.T("filter.linux_rpm")
	case AssetSource:
		return i18n.T("filter.source")
	case AssetAll:
		return i18n.T("filter.all")
	}
	return string(k)
}

// ShortLabel — короткая подпись вида файла.
//
// Используется в карточках сетки: там места мало, и длинные названия
// («Windows MSI (установщик)») обрезаются.
func (k AssetKind) ShortLabel() string {
	switch k {
	case AssetAuto:
		return i18n.T("filter.short.auto")
	case AssetWindowsPortable:
		return i18n.T("filter.short.windows_portable")
	case AssetWindowsMsi:
		return i18n.T("filter.short.windows_msi")
	case AssetWindowsSetup:
		return i18n.T("filter.short.windows_setup")
	case AssetLinuxAppImage:
		return i18n.T("filter.short.linux_appimage")
	case AssetLinuxTarGz:
		return i18n.T("filter.short.linux_targz")
	case AssetLinuxDeb:
		return i18n.T("filter.short.linux_deb")
	case AssetLinuxRpm:
		return i18n.T("filter.short.linux_rpm")
	case AssetSource:
		return i18n.T("filter.short.source")
	case AssetAll:
		return i18n.T("filter.short.all")
	}
	return string(k)
}

// Badge — короткая метка ОС для списка вариантов.
func (k AssetKind) Badge() string {
	switch k {
	case AssetWindowsPortable, AssetWindowsMsi, AssetWindowsSetup:
		return "WIN"
	case AssetLinuxAppImage, AssetLinuxTarGz, AssetLinuxDeb, AssetLinuxRpm:
		return "LNX"
	case AssetSource:
		return "SRC"
	}
	return "···"
}

// ForCurrentOS — виды файлов, подходящие под текущую ОС, в порядке предпочтения.
func ForCurrentOS() []AssetKind {
	switch runtime.GOOS {
	case "windows":
		return []AssetKind{AssetWindowsPortable, AssetWindowsMsi, AssetWindowsSetup}
	case "darwin":
		// macOS-сборок у этих проектов нет: остаётся архив исходников.
		return []AssetKind{AssetLinuxAppImage, AssetLinuxTarGz}
	default:
		return []AssetKind{AssetLinuxAppImage, AssetLinuxTarGz, AssetLinuxDeb, AssetLinuxRpm}
	}
}

// ViewMode — режим отображения списка репозиториев.
type ViewMode string

const (
	// ViewGrid — сетка иконок (режим по умолчанию).
	ViewGrid ViewMode = "Grid"
	// ViewList — иконка слева, описание справа.
	ViewList ViewMode = "List"
	// ViewLight — без картинок.
	ViewLight ViewMode = "Light"
)

// Label возвращает подпись режима.
func (v ViewMode) Label() string {
	switch v {
	case ViewList:
		return i18n.T("repo.view_list")
	case ViewLight:
		return i18n.T("repo.view_light")
	}
	return i18n.T("repo.view_grid")
}

// DownloadMode — как скачиваются релизы.
type DownloadMode string

const (
	// DownloadParallel — файлы и репозитории качаются одновременно.
	DownloadParallel DownloadMode = "Parallel"
	// DownloadSequential — строго по одному файлу за раз.
	DownloadSequential DownloadMode = "Sequential"
)

// Label возвращает подпись режима.
func (d DownloadMode) Label() string {
	if d == DownloadSequential {
		return i18n.T("download.mode_sequential")
	}
	return i18n.T("download.mode_parallel")
}

// RepoConfig — настройки одного репозитория.
type RepoConfig struct {
	Slug    string    `json:"slug"`
	Enabled bool      `json:"enabled"`
	Pinned  []string  `json:"pinned"`
	Filter  AssetKind `json:"filter"`
	// CloneSource — дополнительно клонировать/обновлять исходники через git.
	CloneSource bool   `json:"clone_source"`
	Description string `json:"description"`
	// Purpose — короткое назначение программы: «работа с PDF-документами»,
	// «монтаж видео». Показывается в плитке сетки.
	Purpose string `json:"purpose"`
	// Category — ключ группы, в которой программа показывается в списке:
	// «capture», «media», «graphics», «system», «security», «server».
	Category string  `json:"category"`
	Icon     *string `json:"icon"`
}

// NewRepoConfig создаёт настройки репозитория с выключенной загрузкой.
func NewRepoConfig(slug string) RepoConfig {
	return RepoConfig{
		Slug:     slug,
		Enabled:  false,
		Pinned:   []string{},
		Filter:   AssetAuto,
		Purpose:  DefaultPurposes[slug],
		Category: DefaultCategories[slug],
	}
}

// ShortName — имя репозитория без владельца.
func (r RepoConfig) ShortName() string {
	if i := strings.LastIndex(r.Slug, "/"); i >= 0 {
		return r.Slug[i+1:]
	}
	return r.Slug
}

// Author — владелец репозитория (автор).
func (r RepoConfig) Author() string {
	if i := strings.Index(r.Slug, "/"); i > 0 {
		return r.Slug[:i]
	}
	return "other"
}

// IsPinned сообщает, закреплён ли релиз.
func (r RepoConfig) IsPinned(tag string) bool {
	for _, p := range r.Pinned {
		if p == tag {
			return true
		}
	}
	return false
}

// Config — настройки приложения.
type Config struct {
	// RootDir — куда складываются скачанные версии.
	RootDir string `json:"root_dir"`
	// SpeedLimitKbps — 0 — без ограничения скорости, иначе килобит/с.
	SpeedLimitKbps uint64       `json:"speed_limit_kbps"`
	MaxParallel    int          `json:"max_parallel"`
	DownloadMode   DownloadMode `json:"download_mode"`
	StableChannel  bool         `json:"stable_channel"`
	EdgeChannel    bool         `json:"edge_channel"`
	VerifySHA256   bool         `json:"verify_sha256"`
	// DeleteOldVersions — удалять прошлые версии после успешной загрузки новых.
	DeleteOldVersions bool   `json:"delete_old_versions"`
	Autostart         bool   `json:"autostart"`
	StartMinimized    bool   `json:"start_minimized"`
	CloseToTray       bool   `json:"close_to_tray"`
	GitHubToken       string `json:"github_token"`
	// SelfUpdateRepo — репозиторий с исходниками самого CraftPull.
	SelfUpdateRepo   string `json:"self_update_repo"`
	SelfUpdateBranch string `json:"self_update_branch"`
	// Lang — язык интерфейса: 0 — английский, 1 — русский.
	Lang     uint8        `json:"lang"`
	ViewMode ViewMode     `json:"view_mode"`
	Repos    []RepoConfig `json:"repos"`
}

// Default возвращает настройки по умолчанию.
func Default() Config {
	repos := make([]RepoConfig, 0, len(DefaultRepos))
	for _, s := range DefaultRepos {
		repos = append(repos, NewRepoConfig(s))
	}
	return Config{
		RootDir:           DefaultRootDir(),
		SpeedLimitKbps:    0,
		MaxParallel:       8,
		DownloadMode:      DownloadParallel,
		StableChannel:     true,
		EdgeChannel:       false,
		VerifySHA256:      true,
		DeleteOldVersions: true,
		Autostart:         false,
		StartMinimized:    false,
		CloseToTray:       true,
		GitHubToken:       "",
		SelfUpdateRepo:    "",
		SelfUpdateBranch:  "main",
		Lang:              i18n.EN,
		ViewMode:          ViewGrid,
		Repos:             repos,
	}
}

// DefaultRootDir — папка загрузок по умолчанию.
func DefaultRootDir() string {
	if runtime.GOOS == "windows" {
		if _, err := os.Stat("E:\\Open\\Src"); err == nil {
			return "E:\\Open\\Src"
		}
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, "Downloads", "CraftPull")
		}
		return "CraftPull"
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "CraftPull"
	}
	return filepath.Join(home, "Downloads", "CraftPull")
}

// ConfigPath — путь к файлу конфигурации.
//
// Переменные окружения CRAFTPULL_CONFIG_DIR и CRAFTPULL_DATA_DIR позволяют
// запустить программу в отдельной папке — так тесты не трогают настройки
// пользователя.
func ConfigPath() string {
	if dir := os.Getenv("CRAFTPULL_CONFIG_DIR"); dir != "" {
		return filepath.Join(dir, "config.json")
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "CraftPull", "config.json")
}

// DataDir — папка с данными приложения (логи, иконки, исходники).
func DataDir() string {
	if dir := os.Getenv("CRAFTPULL_DATA_DIR"); dir != "" {
		return dir
	}
	if runtime.GOOS == "windows" {
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			return filepath.Join(local, "CraftPull")
		}
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "CraftPull")
}

// IconsDir — папка с локальным кэшем иконок.
func IconsDir() string { return filepath.Join(DataDir(), "icons") }

// LogPath — путь к файлу журнала.
func LogPath() string { return filepath.Join(DataDir(), "craftpull.log") }

// CachePath — путь к кэшу релизов.
func CachePath() string { return filepath.Join(DataDir(), "releases-cache.json") }

// Load читает настройки с диска; при ошибке возвращает настройки по умолчанию.
func Load() Config {
	path := ConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return Default()
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "CraftPull: the config file is broken (%v), creating a new one\n", err)
		return Default()
	}
	cfg.ensureRepos()
	return cfg
}

// Save записывает настройки на диск.
func (c Config) Save() error {
	path := ConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// ensureRepos добавляет недостающие репозитории и приводит лимит к допустимому.
func (c *Config) ensureRepos() {
	for _, slug := range DefaultRepos {
		found := false
		for _, r := range c.Repos {
			if r.Slug == slug {
				found = true
				break
			}
		}
		if !found {
			c.Repos = append(c.Repos, NewRepoConfig(slug))
		}
	}
	// Назначение программы подставляется, только если его ещё нет:
	// своё значение пользователь мог поменять вручную.
	for i := range c.Repos {
		if c.Repos[i].Purpose == "" {
			c.Repos[i].Purpose = DefaultPurposes[c.Repos[i].Slug]
		}
		// Категория — тоже: у старых конфигов её не было вовсе.
		if c.Repos[i].Category == "" {
			c.Repos[i].Category = DefaultCategories[c.Repos[i].Slug]
		}
	}
	if c.MaxParallel <= 0 {
		c.MaxParallel = 1
	}
	if c.MaxParallel > MaxParallel {
		c.MaxParallel = MaxParallel
	}
	if c.SelfUpdateBranch == "" {
		c.SelfUpdateBranch = "main"
	}
}

// Repo возвращает настройки репозитория по slug.
func (c *Config) Repo(slug string) *RepoConfig {
	for i := range c.Repos {
		if c.Repos[i].Slug == slug {
			return &c.Repos[i]
		}
	}
	return nil
}

// RepoListFile — формат файла со списком репозиториев.
type RepoListFile struct {
	Version    int         `json:"version"`
	ExportedAt string      `json:"exported_at"`
	Repos      []RepoEntry `json:"repos"`
}

// RepoEntry — запись списка репозиториев.
type RepoEntry struct {
	Slug        string `json:"slug"`
	Enabled     bool   `json:"enabled"`
	Description string `json:"description"`
	// Purpose — короткое назначение программы.
	Purpose string `json:"purpose"`
	// Category — ключ группы, в которой программа показана в списке.
	Category    string    `json:"category"`
	Pinned      []string  `json:"pinned"`
	Filter      AssetKind `json:"filter"`
	CloneSource bool      `json:"clone_source"`
}

// ParseList читает список репозиториев из JSON-файла.
func ParseList(data []byte) (*RepoListFile, error) {
	var file RepoListFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, err
	}
	return &file, nil
}

// WriteList записывает список репозиториев в JSON-файл.
func WriteList(path string, list RepoListFile) error {
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// ExportList собирает список репозиториев для экспорта.
func (c Config) ExportList() RepoListFile {
	entries := make([]RepoEntry, 0, len(c.Repos))
	for _, r := range c.Repos {
		entries = append(entries, RepoEntry{
			Slug:        r.Slug,
			Enabled:     r.Enabled,
			Description: r.Description,
			Purpose:     r.Purpose,
			Category:    r.Category,
			Pinned:      append([]string{}, r.Pinned...),
			Filter:      r.Filter,
			CloneSource: r.CloneSource,
		})
	}
	return RepoListFile{Version: 1, Repos: entries}
}

// ImportList сливает список из файла с текущим; возвращает число добавленных.
func (c *Config) ImportList(file *RepoListFile) int {
	added := 0
	for _, entry := range file.Repos {
		slug := strings.TrimSpace(entry.Slug)
		if slug == "" || !strings.Contains(slug, "/") {
			continue
		}
		if existing := c.Repo(slug); existing != nil {
			existing.Enabled = entry.Enabled
			if entry.Description != "" {
				existing.Description = entry.Description
			}
			if entry.Purpose != "" {
				existing.Purpose = entry.Purpose
			}
			if entry.Category != "" {
				existing.Category = entry.Category
			}
			for _, p := range entry.Pinned {
				if !existing.IsPinned(p) {
					existing.Pinned = append(existing.Pinned, p)
				}
			}
			existing.Filter = entry.Filter
			existing.CloneSource = entry.CloneSource
			continue
		}
		c.Repos = append(c.Repos, RepoConfig{
			Slug:        slug,
			Enabled:     entry.Enabled,
			Pinned:      append([]string{}, entry.Pinned...),
			Filter:      entry.Filter,
			CloneSource: entry.CloneSource,
			Description: entry.Description,
			Purpose:     entry.Purpose,
			Category:    entry.Category,
			Icon:        nil,
		})
		added++
	}
	return added
}
