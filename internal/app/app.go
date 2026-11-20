// Package app — состояние, общее для любого интерфейса: выбор файлов,
// открытая панель релизов, режим отображения, язык.
//
// Логика не зависит от библиотеки отрисовки: её используют и графическое
// окно, и терминальный вариант, и тесты.
package app

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/KosenkovPro/craftpull-go/internal/config"
	"github.com/KosenkovPro/craftpull-go/internal/i18n"
	"github.com/KosenkovPro/craftpull-go/internal/manager"
	"github.com/KosenkovPro/craftpull-go/internal/model"
)

// Tab — вкладка приложения.
type Tab int

const (
	// TabRepos — список репозиториев.
	TabRepos Tab = iota
	// TabSettings — настройки.
	TabSettings
	// TabLogs — журнал.
	TabLogs
)

// State — состояние интерфейса.
type State struct {
	mu sync.RWMutex

	tab  Tab
	view config.ViewMode
	lang uint8

	// releasesOpen — открыта ли панель релизов справа.
	releasesOpen bool
	// releasesSlug — репозиторий, чьи релизы открыты.
	releasesSlug string
	// expanded — раскрытый релиз (тег).
	expanded string
	// selected — отмеченные файлы в формате «тег::имя».
	selected map[string]bool
	// preselected — релизы, для которых файлы уже отмечены автоматически.
	preselected map[string]bool
	// hero — раскрытая карточка в сетке.
	hero string
	// notice — всплывающее сообщение.
	notice string
}

// New создаёт состояние интерфейса.
func New(view config.ViewMode, lang uint8) *State {
	return &State{
		tab:         TabRepos,
		view:        view,
		lang:        lang,
		selected:    make(map[string]bool),
		preselected: make(map[string]bool),
	}
}

// Tab возвращает активную вкладку.
func (s *State) Tab() Tab {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tab
}

// SetTab переключает вкладку.
func (s *State) SetTab(tab Tab) {
	s.mu.Lock()
	s.tab = tab
	s.mu.Unlock()
}

// View возвращает режим отображения.
func (s *State) View() config.ViewMode {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.view
}

// SetView меняет режим отображения.
func (s *State) SetView(v config.ViewMode) {
	s.mu.Lock()
	s.view = v
	s.mu.Unlock()
}

// Lang возвращает язык интерфейса.
func (s *State) Lang() uint8 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lang
}

// SetLang меняет язык интерфейса.
func (s *State) SetLang(lang uint8) {
	s.mu.Lock()
	s.lang = lang
	s.mu.Unlock()
	i18n.Set(lang)
}

// ReleasesOpen сообщает, открыта ли панель релизов.
func (s *State) ReleasesOpen() (bool, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.releasesOpen, s.releasesSlug
}

// OpenReleases открывает панель релизов репозитория.
func (s *State) OpenReleases(slug string) {
	s.mu.Lock()
	s.releasesOpen = true
	s.releasesSlug = slug
	s.mu.Unlock()
}

// CloseReleases закрывает панель релизов.
func (s *State) CloseReleases() {
	s.mu.Lock()
	s.releasesOpen = false
	s.releasesSlug = ""
	s.expanded = ""
	s.mu.Unlock()
}

// Expanded возвращает раскрытый релиз.
func (s *State) Expanded() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.expanded
}

// ToggleExpand раскрывает или сворачивает релиз.
func (s *State) ToggleExpand(tag string) {
	s.mu.Lock()
	if s.expanded == tag {
		s.expanded = ""
	} else {
		s.expanded = tag
	}
	s.mu.Unlock()
}

// SetExpanded раскрывает релиз (без переключения).
func (s *State) SetExpanded(tag string) {
	s.mu.Lock()
	s.expanded = tag
	s.mu.Unlock()
}

// Hero возвращает раскрытую карточку сетки.
func (s *State) Hero() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.hero
}

// SetHero задаёт раскрытую карточку сетки.
func (s *State) SetHero(slug string) {
	s.mu.Lock()
	s.hero = slug
	s.mu.Unlock()
}

// Notice возвращает всплывающее сообщение.
func (s *State) Notice() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.notice
}

// SetNotice задаёт всплывающее сообщение.
func (s *State) SetNotice(text string) {
	s.mu.Lock()
	s.notice = text
	s.mu.Unlock()
}

// AssetKey возвращает ключ отмеченного файла.
func AssetKey(tag, name string) string { return tag + "::" + name }

// IsSelected сообщает, отмечен ли файл.
func (s *State) IsSelected(tag, name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.selected[AssetKey(tag, name)]
}

// ToggleAsset отмечает или снимает отметку файла.
func (s *State) ToggleAsset(tag, name string) {
	key := AssetKey(tag, name)
	s.mu.Lock()
	if s.selected[key] {
		delete(s.selected, key)
	} else {
		s.selected[key] = true
	}
	s.mu.Unlock()
}

// SelectRelease отмечает все файлы релиза.
func (s *State) SelectRelease(tag string, assets []model.AssetInfo) {
	s.mu.Lock()
	for _, a := range assets {
		s.selected[AssetKey(tag, a.Name)] = true
	}
	s.mu.Unlock()
}

// ClearSelection снимает все отметки.
func (s *State) ClearSelection() {
	s.mu.Lock()
	s.selected = make(map[string]bool)
	s.mu.Unlock()
}

// SelectionSummary — итог по отмеченным файлам: сколько, какого размера, релиз.
func (s *State) SelectionSummary(releases []model.ReleaseInfo) (int, string, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	var bytes uint64
	tag := ""
	for _, rel := range releases {
		for _, asset := range rel.Assets {
			if s.selected[AssetKey(rel.Tag, asset.Name)] {
				count++
				bytes += asset.Size
				if tag == "" {
					tag = rel.Tag
				}
			}
		}
	}
	size := model.HumanSize(bytes)
	if count == 0 {
		size = i18n.T("releases.none")
	}
	return count, size, tag
}

// CollectSelection собирает отмеченные файлы по релизам.
func (s *State) CollectSelection(releases []model.ReleaseInfo) []model.AssetInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.AssetInfo, 0, 8)
	for _, rel := range releases {
		for _, asset := range rel.Assets {
			if s.selected[AssetKey(rel.Tag, asset.Name)] {
				out = append(out, asset)
			}
		}
	}
	return out
}

// Preselect отмечает файл, подходящий под текущую ОС.
//
// Отметка ставится один раз на релиз: иначе снятие выделения пользователем
// тут же возвращалось бы при следующей перерисовке.
func (s *State) Preselect(slug string, rel *model.ReleaseInfo) {
	options := manager.AssetOptions(slug, rel)
	if len(options) == 0 {
		return
	}
	preferred := config.ForCurrentOS()
	chosen := options[0]
	for _, opt := range options {
		for _, p := range preferred {
			if opt.Kind == p {
				chosen = opt
				break
			}
		}
	}
	s.mu.Lock()
	if s.preselected[rel.Tag] {
		s.mu.Unlock()
		return
	}
	s.preselected[rel.Tag] = true
	s.selected[AssetKey(rel.Tag, chosen.Asset.Name)] = true
	s.mu.Unlock()
}

// SelectedTags возвращает теги релизов, в которых есть отметки.
func (s *State) SelectedTags(releases []model.ReleaseInfo) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	seen := make(map[string]bool)
	out := make([]string, 0, 4)
	for _, rel := range releases {
		for _, asset := range rel.Assets {
			if s.selected[AssetKey(rel.Tag, asset.Name)] && !seen[rel.Tag] {
				seen[rel.Tag] = true
				out = append(out, rel.Tag)
			}
		}
	}
	sort.Strings(out)
	return out
}

// SummaryText собирает текст итога для панели релизов.
func (s *State) SummaryText(releases []model.ReleaseInfo) string {
	count, size, tag := s.SelectionSummary(releases)
	word := i18n.Plural(count, "файл", "файла", "файлов")
	if i18n.Current() == i18n.EN {
		word = i18n.Plural(count, "file", "files", "files")
	}
	return i18n.TF("releases.summary", map[string]string{
		"{n}": fmt.Sprintf("%d", count),
		"{f}": word,
		"{s}": size,
		"{t}": tag,
	})
}

// FilterOptions собирает варианты файла для репозитория по последнему релизу.
func FilterOptions(slug string, releases []model.ReleaseInfo) []manager.AssetOption {
	if len(releases) == 0 {
		return nil
	}
	return manager.AssetOptions(slug, &releases[0])
}

// FilterLabel возвращает подпись выбранного вида файла.
func FilterLabel(kind config.AssetKind) string {
	if kind == config.AssetAuto {
		return i18n.T("filter.auto")
	}
	return kind.Label()
}

// NormalizeSlug приводит «владелец/репозиторий» к каноничному виду.
func NormalizeSlug(slug string) string {
	return strings.ToLower(strings.TrimSpace(slug))
}

// ValidSlug проверяет, что строка похожа на «владелец/репозиторий».
func ValidSlug(slug string) bool {
	slug = NormalizeSlug(slug)
	if slug == "" || !strings.Contains(slug, "/") {
		return false
	}
	parts := strings.Split(slug, "/")
	if len(parts) != 2 {
		return false
	}
	for _, p := range parts {
		if p == "" {
			return false
		}
	}
	return true
}
