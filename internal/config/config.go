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
var DefaultRepos = []string{
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
	CloneSource bool    `json:"clone_source"`
	Description string  `json:"description"`
	Icon        *string `json:"icon"`
}

// NewRepoConfig создаёт настройки репозитория с выключенной загрузкой.
func NewRepoConfig(slug string) RepoConfig {
	return RepoConfig{Slug: slug, Enabled: false, Pinned: []string{}, Filter: AssetAuto}
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
	Slug        string    `json:"slug"`
	Enabled     bool      `json:"enabled"`
	Description string    `json:"description"`
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
			Icon:        nil,
		})
		added++
	}
	return added
}
