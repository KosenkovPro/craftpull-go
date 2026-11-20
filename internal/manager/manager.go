// Package manager — планирование загрузок, выбор файлов релиза, уборка старых версий.
package manager

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/KosenkovPro/craftpull-go/internal/cache"
	"github.com/KosenkovPro/craftpull-go/internal/config"
	"github.com/KosenkovPro/craftpull-go/internal/download"
	"github.com/KosenkovPro/craftpull-go/internal/gh"
	"github.com/KosenkovPro/craftpull-go/internal/i18n"
	"github.com/KosenkovPro/craftpull-go/internal/model"
	"github.com/KosenkovPro/craftpull-go/internal/storage"
)

// CheckStatus — как проходит проверка репозитория.
type CheckStatus int

const (
	// CheckWaiting — ждёт своей очереди.
	CheckWaiting CheckStatus = iota
	// CheckRunning — проверяется прямо сейчас.
	CheckRunning
	// CheckDone — проверка прошла успешно.
	CheckDone
	// CheckFailed — проверка закончилась ошибкой.
	CheckFailed
	// CheckSkipped — проверка не понадобилась: данные свежие.
	CheckSkipped
)

// CheckItem — один пункт списка проверок.
type CheckItem struct {
	Slug   string      `json:"slug"`
	Status CheckStatus `json:"status"`
	// Detail — итог проверки: «10 релизов · 0.9 с».
	Detail string  `json:"detail"`
	Error  *string `json:"error"`
}

// CheckState — сквозная проверка репозиториев.
type CheckState struct {
	Running bool        `json:"running"`
	Items   []CheckItem `json:"items"`
}

// Total — сколько всего репозиториев в проверке.
func (c CheckState) Total() int { return len(c.Items) }

// Done — сколько проверок уже закончилось.
func (c CheckState) Done() int {
	n := 0
	for _, item := range c.Items {
		switch item.Status {
		case CheckDone, CheckFailed, CheckSkipped:
			n++
		}
	}
	return n
}

// Fraction — доля выполненных проверок.
func (c CheckState) Fraction() float32 {
	if len(c.Items) == 0 {
		return 0
	}
	return float32(c.Done()) / float32(len(c.Items))
}

// DownloadRequest — что именно скачиваем.
type DownloadRequest struct {
	// Tag — тег релиза; пустой означает «последние релизы по каналам».
	Tag string
	// Assets — явно выбранные пользователем файлы релиза.
	Assets []model.AssetInfo
	// Targets — скачивать целевые релизы по включённым каналам.
	Targets bool
}

// RepoRuntime — состояние репозитория во время работы программы.
type RepoRuntime struct {
	Slug         string
	Releases     []model.ReleaseInfo
	LatestStable *model.ReleaseInfo
	LatestEdge   *model.ReleaseInfo
	Installed    []model.InstalledVersion
	LastCheck    *string
	LastError    *string
	Checking     bool
}

// Manager — общее состояние приложения и вся работа с репозиториями.
type Manager struct {
	mu       sync.RWMutex
	cfg      config.Config
	client   *gh.Client
	tasks    *download.TaskRegistry
	control  *download.Control
	limiter  *download.RateLimiter
	seq      atomic.Uint64
	busy     atomic.Int64
	repos    map[string]*RepoRuntime
	logs     []model.LogEntry
	logFile  *os.File
	checks   CheckState
	checked  map[string]time.Time
	parallel chan struct{}
	cache    *cache.Cache
}

// New создаёт менеджер с настройками по умолчанию.
func New(cfg config.Config) (*Manager, error) {
	client, err := gh.NewClient(cfg.GitHubToken)
	if err != nil {
		return nil, err
	}
	m := &Manager{
		cfg:     cfg,
		client:  client,
		tasks:   download.NewTaskRegistry(),
		control: download.NewControl(),
		limiter: download.NewRateLimiter(),
		repos:   make(map[string]*RepoRuntime),
		checked: make(map[string]time.Time),
		cache:   cache.New(config.CachePath()),
	}
	m.limiter.SetLimit(cfg.SpeedLimitKbps * 1024)
	m.parallel = make(chan struct{}, max(cfg.MaxParallel, 1))
	m.openLog()
	m.loadCache()
	return m, nil
}

// loadCache поднимает релизы из кэша: программа сразу показывает данные,
// даже если сеть недоступна или лимит GitHub API исчерпан.
func (m *Manager) loadCache() {
	for _, r := range m.cfg.Repos {
		releases, ok := m.cache.Get(r.Slug)
		if !ok {
			continue
		}
		st, edge := SplitChannels(releases)
		m.repos[r.Slug] = &RepoRuntime{
			Slug:         r.Slug,
			Releases:     releases,
			LatestStable: st,
			LatestEdge:   edge,
			Installed:    storage.ScanInstalled(m.cfg.RootDir, r.Slug),
		}
		if updated, ok := m.cache.UpdatedAt(r.Slug); ok {
			m.checked[r.Slug] = updated
		}
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Close освобождает ресурсы.
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.logFile != nil {
		_ = m.logFile.Close()
		m.logFile = nil
	}
}

// openLog открывает файл журнала (ошибка не критична).
func (m *Manager) openLog() {
	path := config.LogPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	m.logFile = f
}

// Log записывает строку в журнал и в файл лога.
func (m *Manager) Log(level model.LogLevel, text string) {
	entry := model.LogEntry{
		Time:  time.Now().Format("15:04:05"),
		Level: level,
		Text:  text,
	}
	m.mu.Lock()
	m.logs = append(m.logs, entry)
	if len(m.logs) > 2000 {
		m.logs = m.logs[len(m.logs)-2000:]
	}
	file := m.logFile
	m.mu.Unlock()

	if file != nil {
		line := fmt.Sprintf("%s [%d] %s\n", entry.Time, level, entry.Text)
		_, _ = file.WriteString(line)
	}
}

// Logs возвращает копию журнала.
func (m *Manager) Logs() []model.LogEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]model.LogEntry, len(m.logs))
	copy(out, m.logs)
	return out
}

// ClearLogs очищает журнал.
func (m *Manager) ClearLogs() {
	m.mu.Lock()
	m.logs = nil
	m.mu.Unlock()
}

// Config возвращает текущие настройки.
func (m *Manager) Config() config.Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cfg
}

// SaveConfig сохраняет настройки на диск и применяет их к менеджеру.
func (m *Manager) SaveConfig(cfg config.Config) error {
	if err := cfg.Save(); err != nil {
		return errors.New(i18n.TF1("err.cfg_save", "{e}", err.Error()))
	}
	m.mu.Lock()
	m.cfg = cfg
	m.mu.Unlock()
	m.limiter.SetLimit(cfg.SpeedLimitKbps * 1024)
	m.mu.Lock()
	m.parallel = make(chan struct{}, max(cfg.MaxParallel, 1))
	m.mu.Unlock()
	if cfg.GitHubToken != m.client.Token() {
		if client, err := gh.NewClient(cfg.GitHubToken); err == nil {
			m.mu.Lock()
			m.client = client
			m.mu.Unlock()
		}
	}
	return nil
}

// Tasks возвращает реестр задач.
func (m *Manager) Tasks() *download.TaskRegistry { return m.tasks }

// Control возвращает контроллер паузы/остановки.
func (m *Manager) Control() *download.Control { return m.control }

// IsBusy сообщает, идут ли загрузки или проверки.
func (m *Manager) IsBusy() bool { return m.busy.Load() > 0 }

// WaitIdle ждёт, пока закончатся все фоновые задачи.
func (m *Manager) WaitIdle(ctx context.Context) {
	for m.IsBusy() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// HasErrors сообщает, есть ли среди задач ошибки.
func (m *Manager) HasErrors() bool {
	for _, t := range m.tasks.All() {
		if t.State == model.TaskFailed {
			return true
		}
	}
	return false
}

// RepoRuntime возвращает состояние репозитория.
func (m *Manager) RepoRuntime(slug string) (RepoRuntime, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rt, ok := m.repos[slug]
	if !ok {
		return RepoRuntime{}, false
	}
	return *rt, true
}

// Releases возвращает релизы репозитория из памяти.
func (m *Manager) Releases(slug string) []model.ReleaseInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	rt, ok := m.repos[slug]
	if !ok {
		return nil
	}
	return rt.Releases
}

// CachedReleases возвращает релизы из памяти.
func (m *Manager) CachedReleases(slug string) ([]model.ReleaseInfo, bool) {
	rt, ok := m.RepoRuntime(slug)
	if !ok || len(rt.Releases) == 0 {
		return nil, false
	}
	return rt.Releases, true
}

// IsStale сообщает, устарели ли данные репозитория.
func (m *Manager) IsStale(slug string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	last, ok := m.checked[slug]
	if !ok {
		return true
	}
	return time.Since(last) > 30*time.Minute
}

// RefreshRepo обновляет данные одного репозитория; возвращает число релизов.
func (m *Manager) RefreshRepo(ctx context.Context, slug string) (int, error) {
	m.mu.Lock()
	rt, ok := m.repos[slug]
	if !ok {
		rt = &RepoRuntime{Slug: slug}
		m.repos[slug] = rt
	}
	rt.Checking = true
	rt.LastError = nil
	client := m.client
	m.mu.Unlock()

	releases, err := client.ListReleases(ctx, slug)

	m.mu.Lock()
	rt = m.repos[slug]
	rt.Checking = false
	if err != nil {
		msg := err.Error()
		rt.LastError = &msg
		m.checked[slug] = time.Now()
		m.mu.Unlock()
		return 0, err
	}
	rt.Releases = releases
	st, edge := SplitChannels(releases)
	rt.LatestStable = st
	rt.LatestEdge = edge
	now := time.Now().Format(time.RFC3339)
	rt.LastCheck = &now
	m.checked[slug] = time.Now()
	root := m.cfg.RootDir
	m.mu.Unlock()

	// Метаданные пишемся на диск без блокировки — они читаются чаще.
	rt.Installed = storage.ScanInstalled(root, slug)
	m.cache.Put(slug, releases)
	return len(releases), nil
}

// RefreshAll проверяет все включённые репозитории и ведёт список проверок.
func (m *Manager) RefreshAll(ctx context.Context, force bool) {
	// Список собирается без мьютекса: IsStale тоже берёт мьютекс, и вызов
	// внутри заблокированной секции приводил бы к взаимоблокировке.
	m.mu.Lock()
	enabled := make([]string, 0, len(m.cfg.Repos))
	for _, r := range m.cfg.Repos {
		if r.Enabled {
			enabled = append(enabled, r.Slug)
		}
	}
	m.mu.Unlock()

	slugs := make([]string, 0, len(enabled))
	for _, slug := range enabled {
		if force || m.IsStale(slug) {
			slugs = append(slugs, slug)
		}
	}

	items := make([]CheckItem, 0, len(slugs))
	for _, s := range slugs {
		items = append(items, CheckItem{Slug: s, Status: CheckWaiting})
	}
	m.mu.Lock()
	m.checks = CheckState{Running: len(slugs) > 0, Items: items}
	m.mu.Unlock()

	if len(slugs) == 0 {
		return
	}
	m.busy.Add(1)
	defer m.busy.Add(-1)

	for i, slug := range slugs {
		if ctx.Err() != nil {
			return
		}
		m.setCheckStatus(i, CheckRunning, "", nil)
		start := time.Now()
		count, err := m.RefreshRepo(ctx, slug)
		elapsed := time.Since(start).Seconds()
		if err != nil {
			errMsg := err.Error()
			m.setCheckStatus(i, CheckFailed, i18n.TF1("check.failed", "{t}", fmt.Sprintf("%.1f", elapsed)), &errMsg)
			m.Log(model.LogError, fmt.Sprintf("%s: %s", slug, err.Error()))
			continue
		}
		m.setCheckStatus(i, CheckDone,
			i18n.TF("check.done", map[string]string{
				"{n}": fmt.Sprintf("%d", count),
				"{t}": fmt.Sprintf("%.1f", elapsed),
			}), nil)
	}
	m.mu.Lock()
	m.checks.Running = false
	m.mu.Unlock()
}

// setCheckStatus обновляет пункт списка проверок.
func (m *Manager) setCheckStatus(index int, status CheckStatus, detail string, errMsg *string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if index < 0 || index >= len(m.checks.Items) {
		return
	}
	m.checks.Items[index].Status = status
	m.checks.Items[index].Detail = detail
	m.checks.Items[index].Error = errMsg
}

// Checks возвращает состояние списка проверок.
func (m *Manager) Checks() CheckState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.checks
}

// PlanTargets — какие релизы считаем целями для репозитория.
func (m *Manager) PlanTargets(slug string) []model.ReleaseInfo {
	cfg := m.Config()
	rt, ok := m.RepoRuntime(slug)
	if !ok {
		return nil
	}
	out := make([]model.ReleaseInfo, 0, 2)
	if cfg.StableChannel && rt.LatestStable != nil {
		out = append(out, *rt.LatestStable)
	}
	if cfg.EdgeChannel && rt.LatestEdge != nil {
		out = append(out, *rt.LatestEdge)
	}
	seen := make(map[string]bool)
	unique := out[:0]
	for _, r := range out {
		if seen[r.Tag] {
			continue
		}
		seen[r.Tag] = true
		unique = append(unique, r)
	}
	return unique
}

// DownloadRepo скачивает целевые релизы репозитория и убирает старые версии.
//
// Флаг отмены здесь не сбрасывается: он живёт на весь заход, иначе «Стоп»
// прерывал бы только один репозиторий, а следующий тут же стартовал бы заново.
func (m *Manager) DownloadRepo(ctx context.Context, slug string, request DownloadRequest) {
	cfg := m.Config()
	filter := config.AssetAuto
	if rc := cfg.Repo(slug); rc != nil {
		filter = rc.Filter
	}

	rt, ok := m.RepoRuntime(slug)
	if !ok || len(rt.Releases) == 0 {
		if _, err := m.RefreshRepo(ctx, slug); err != nil {
			m.Log(model.LogError, fmt.Sprintf("%s: %s", slug, err.Error()))
		}
		rt, _ = m.RepoRuntime(slug)
	}

	// План: что и какими файлами качаем.
	type planItem struct {
		release model.ReleaseInfo
		assets  []model.AssetInfo
	}
	var plan []planItem
	if request.Targets {
		for _, target := range m.PlanTargets(slug) {
			target := target
			plan = append(plan, planItem{target, SelectAssets(slug, filter, &target)})
		}
	} else {
		for _, rel := range rt.Releases {
			if rel.Tag == request.Tag {
				plan = append(plan, planItem{rel, request.Assets})
				break
			}
		}
	}

	if len(plan) == 0 {
		m.Log(model.LogWarn, i18n.TF1("log.no_releases", "{slug}", slug))
		return
	}

	root := cfg.RootDir
	downloadedTags := make([]string, 0, 2)
	totalFailures := 0

	for _, item := range plan {
		target := item.release
		assets := item.assets
		tag := target.Tag
		channel := target.Channel()
		dir := storage.VersionDir(root, slug, tag)

		if len(assets) == 0 {
			m.Log(model.LogWarn, i18n.TF("log.no_assets", map[string]string{
				"{slug}": slug,
				"{tag}":  tag,
			}))
			continue
		}

		// «Стоп» — дальше не идём.
		if m.control.IsCancelled() {
			break
		}

		// Уже скачано?
		installed := storage.ScanInstalled(root, slug)
		already := false
		for _, v := range installed {
			if v.Tag == tag && len(v.Files) > 0 {
				already = true
				break
			}
		}
		if already {
			m.Log(model.LogInfo, i18n.TF("log.already", map[string]string{
				"{slug}": slug,
				"{tag}":  tag,
			}))
			downloadedTags = append(downloadedTags, tag)
			continue
		}

		_ = os.MkdirAll(dir, 0o755)
		_ = storage.WriteMeta(dir, storage.VersionMeta{
			Tag:         tag,
			Channel:     channel.Label(),
			InstalledAt: time.Now().Format(time.RFC3339),
		})

		m.Log(model.LogInfo, i18n.TF("log.downloading", map[string]string{
			"{slug}": slug,
			"{tag}":  tag,
			"{size}": channel.Label(),
			"{n}":    fmt.Sprintf("%d", len(assets)),
		}))

		failures := 0
		if cfg.DownloadMode == config.DownloadSequential {
			// Последовательно: строго один файл за раз.
			for _, asset := range assets {
				if m.control.IsCancelled() {
					break
				}
				dl := m.newDownloader()
				id := download.TaskID(slug, tag, asset.Name)
				if err := dl.Fetch(ctx, id, slug, tag, asset, dir); err != nil {
					failures++
					if m.control.IsCancelled() {
						m.Log(model.LogWarn, i18n.T("log.stopped"))
					} else {
						m.Log(model.LogError, fmt.Sprintf("%s %s: %s — %s", slug, tag, asset.Name, err.Error()))
					}
					continue
				}
				m.Log(model.LogOk, i18n.TF("log.ready", map[string]string{
					"{slug}": slug,
					"{tag}":  tag,
					"{name}": asset.Name,
				}))
			}
		} else {
			// Параллельно: файлы релиза качаются одновременно, но не больше
			// лимита одновременных загрузок.
			var wg sync.WaitGroup
			for _, asset := range assets {
				asset := asset
				wg.Add(1)
				go func() {
					defer wg.Done()
					// Ожидание места в очереди тоже должно прерываться «Стопом».
					select {
					case <-m.control.Done():
						return
					case <-ctx.Done():
						return
					case m.parallel <- struct{}{}:
					}
					defer func() { <-m.parallel }()

					dl := m.newDownloader()
					id := download.TaskID(slug, tag, asset.Name)
					if err := dl.Fetch(ctx, id, slug, tag, asset, dir); err != nil {
						if m.control.IsCancelled() {
							m.Log(model.LogWarn, i18n.T("log.stopped"))
						} else {
							m.Log(model.LogError, fmt.Sprintf("%s %s: %s — %s", slug, tag, asset.Name, err.Error()))
						}
						return
					}
					m.Log(model.LogOk, i18n.TF("log.ready", map[string]string{
						"{slug}": slug,
						"{tag}":  tag,
						"{name}": asset.Name,
					}))
				}()
			}
			wg.Wait()
			failures = m.countFailures(slug, tag, len(assets))
		}
		totalFailures += failures

		if failures == 0 {
			downloadedTags = append(downloadedTags, tag)
		}
	}

	// Уборка старых версий — только если новые действительно скачались.
	if totalFailures == 0 && len(downloadedTags) > 0 {
		m.CleanupRepo(slug, downloadedTags)
	} else if totalFailures > 0 {
		m.Log(model.LogWarn, i18n.TF("log.cleanup_skipped", map[string]string{
			"{slug}": slug,
			"{n}":    fmt.Sprintf("%d", totalFailures),
		}))
	}
}

// countFailures считает неудачные задачи конкретного релиза.
func (m *Manager) countFailures(slug, tag string, total int) int {
	failures := 0
	for _, t := range m.tasks.All() {
		if t.Repo == slug && t.Tag == tag && t.State == model.TaskFailed {
			failures++
		}
	}
	if failures > total {
		failures = total
	}
	return failures
}

// newDownloader собирает загрузчик с текущими настройками.
func (m *Manager) newDownloader() *download.Downloader {
	cfg := m.Config()
	return &download.Downloader{
		Client: &http.Client{
			Timeout: 0,
			Transport: &http.Transport{
				MaxIdleConns:        32,
				MaxIdleConnsPerHost: 8,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		Limiter:      m.limiter,
		Control:      m.control,
		Tasks:        m.tasks,
		VerifySHA256: cfg.VerifySHA256,
		Seq:          m.seq.Add(1) - 1,
	}
}

// CleanupRepo убирает старые версии репозитория.
func (m *Manager) CleanupRepo(slug string, keep []string) {
	cfg := m.Config()
	if !cfg.DeleteOldVersions {
		return
	}
	keepSet := make(map[string]bool, len(keep))
	for _, k := range keep {
		keepSet[k] = true
	}
	var pinned []string
	if rc := cfg.Repo(slug); rc != nil {
		pinned = rc.Pinned
	}
	removed := storage.CleanupVersions(cfg.RootDir, slug, keepSet, pinned)
	for _, path := range removed {
		m.Log(model.LogInfo, i18n.TF("log.deleted", map[string]string{
			"{slug}": slug,
			"{path}": path,
		}))
	}
}

// RemoveVersion удаляет скачанную версию репозитория.
//
// Закреплённые версии удалить нельзя: они хранятся в настройках репозитория.
func (m *Manager) RemoveVersion(slug, tag string) bool {
	cfg := m.Config()
	rc := cfg.Repo(slug)
	if rc == nil {
		return false
	}
	return storage.RemoveVersion(cfg.RootDir, slug, tag, rc.Pinned)
}

// DownloadAll скачивает новые релизы всех включённых репозиториев.
func (m *Manager) DownloadAll(ctx context.Context) {
	cfg := m.Config()
	slugs := make([]string, 0, len(cfg.Repos))
	for _, r := range cfg.Repos {
		if r.Enabled {
			slugs = append(slugs, r.Slug)
		}
	}
	if len(slugs) == 0 {
		m.Log(model.LogWarn, i18n.T("log.no_repo"))
		return
	}
	m.control.Reset()
	m.busy.Add(1)
	defer m.busy.Add(-1)

	var wg sync.WaitGroup
	for _, slug := range slugs {
		slug := slug
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.DownloadRepo(ctx, slug, DownloadRequest{Targets: true})
		}()
	}
	wg.Wait()
}

// DownloadFresh скачивает только то, чего ещё нет на диске.
func (m *Manager) DownloadFresh(ctx context.Context, slug string, request DownloadRequest) {
	m.DownloadRepo(ctx, slug, request)
}

// StopAll останавливает все загрузки.
func (m *Manager) StopAll() {
	m.control.Cancel()
	m.Log(model.LogWarn, i18n.T("log.stopped"))
}

// TogglePause переключает паузу; возвращает новое состояние.
func (m *Manager) TogglePause() bool {
	paused := m.control.TogglePause()
	if paused {
		m.Log(model.LogWarn, i18n.T("notify.paused"))
	} else {
		m.Log(model.LogWarn, i18n.T("notify.resumed"))
	}
	return paused
}

// SplitChannels делит релизы на стабильный и нестабильный.
func SplitChannels(releases []model.ReleaseInfo) (*model.ReleaseInfo, *model.ReleaseInfo) {
	var stable, edge *model.ReleaseInfo
	for i := range releases {
		r := releases[i]
		if r.IsEdge() {
			if edge == nil {
				edge = &r
			}
			continue
		}
		if stable == nil {
			stable = &r
		}
	}
	return stable, edge
}

// IsNoiseAsset — служебный файл релиза: подпись, контрольная сумма, манифест.
func IsNoiseAsset(name string) bool {
	n := strings.ToLower(name)
	for _, suffix := range []string{
		".asc", ".sig", ".zsync", ".sha256", ".sha256sum", ".sha1", ".sha512",
		".minisig", ".blockmap", ".json", ".txt", ".md",
	} {
		if strings.HasSuffix(n, suffix) {
			return true
		}
	}
	return strings.Contains(n, "sha256sum") || strings.Contains(n, "signing-key")
}

// AssetKindOf определяет вид файла по его имени.
//
// Сборки не под x86_64 и служебные файлы отбрасываются: пользователю нужна
// одна конкретная сборка.
func AssetKindOf(name string) (config.AssetKind, bool) {
	n := strings.ToLower(name)
	if IsNoiseAsset(n) {
		return "", false
	}
	// Архитектура: интересует только x86_64.
	for _, arch := range []string{"arm64", "aarch64", "riscv64", "i386", "i686"} {
		if strings.Contains(n, arch) {
			return "", false
		}
	}
	if strings.Contains(n, "windows") || strings.HasSuffix(n, ".msi") || strings.Contains(n, "setup.exe") {
		switch {
		case strings.HasSuffix(n, ".msi"):
			return config.AssetWindowsMsi, true
		case strings.Contains(n, "setup.exe"):
			return config.AssetWindowsSetup, true
		case strings.HasSuffix(n, ".zip"):
			return config.AssetWindowsPortable, true
		}
		return config.AssetWindowsSetup, true
	}
	if strings.Contains(n, "linux") || strings.Contains(n, "ubuntu") {
		switch {
		case strings.HasSuffix(n, ".appimage"):
			return config.AssetLinuxAppImage, true
		case strings.HasSuffix(n, ".tar.gz"), strings.HasSuffix(n, ".tgz"):
			return config.AssetLinuxTarGz, true
		case strings.HasSuffix(n, ".deb"):
			return config.AssetLinuxDeb, true
		case strings.HasSuffix(n, ".rpm"):
			return config.AssetLinuxRpm, true
		}
	}
	return "", false
}

// MainProductPrefix — основной продукт релиза: «pdfcraft-0.21.1-…»,
// а не «pdfcraft-cli-0.21.1-…».
func MainProductPrefix(slug, tag string) string {
	short := storage.RepoName(slug)
	ver := strings.TrimPrefix(tag, "v")
	version := ver
	for i, c := range ver {
		if c >= '0' && c <= '9' {
			version = ver[i:]
			break
		}
	}
	return fmt.Sprintf("%s-%s", short, version)
}

// pickAsset — файл релиза по виду: сначала основной продукт, потом любой подходящий.
func pickAsset(slug string, rel *model.ReleaseInfo, kind config.AssetKind) []model.AssetInfo {
	main := MainProductPrefix(slug, rel.Tag)
	for _, a := range rel.Assets {
		if k, ok := AssetKindOf(a.Name); ok && k == kind && strings.HasPrefix(a.Name, main) {
			return []model.AssetInfo{a}
		}
	}
	for _, a := range rel.Assets {
		if k, ok := AssetKindOf(a.Name); ok && k == kind {
			return []model.AssetInfo{a}
		}
	}
	return nil
}

// SelectAssets выбирает файлы релиза по правилу фильтра.
func SelectAssets(slug string, kind config.AssetKind, rel *model.ReleaseInfo) []model.AssetInfo {
	switch kind {
	case config.AssetAll:
		return append([]model.AssetInfo{}, rel.Assets...)
	case config.AssetSource:
		return []model.AssetInfo{model.SourceZip(rel.Tag, rel.SourceZip)}
	case config.AssetAuto:
		k := OSDefaultKind(slug, rel)
		if k == config.AssetSource {
			return []model.AssetInfo{model.SourceZip(rel.Tag, rel.SourceZip)}
		}
		return pickAsset(slug, rel, k)
	}
	return pickAsset(slug, rel, kind)
}

// AssetOption — вариант файла релиза: вид и сам файл.
type AssetOption struct {
	Kind  config.AssetKind
	Asset model.AssetInfo
}

// AssetOptions — варианты файлов релиза для выпадающего списка.
//
// Один вид — один вариант: если в релизе лежат две сборки одной системы,
// берём основной продукт, а не вспомогательный (CLI, веб).
func AssetOptions(slug string, rel *model.ReleaseInfo) []AssetOption {
	main := MainProductPrefix(slug, rel.Tag)
	out := make([]AssetOption, 0, 6)
	extra := make([]AssetOption, 0, 6)
	for _, a := range rel.Assets {
		k, ok := AssetKindOf(a.Name)
		if !ok || k == config.AssetSource {
			continue
		}
		if strings.HasPrefix(a.Name, main) {
			if !hasKind(out, k) {
				out = append(out, AssetOption{k, a})
			}
			continue
		}
		if !hasKind(extra, k) {
			extra = append(extra, AssetOption{k, a})
		}
	}
	// Варианты основного продукта идут первыми, вспомогательные — после.
	for _, opt := range extra {
		if !hasKind(out, opt.Kind) {
			out = append(out, opt)
		}
	}
	// Ставим варианты текущей ОС первыми — их чаще выбирают.
	preferred := config.ForCurrentOS()
	sort.SliceStable(out, func(i, j int) bool {
		return osRank(preferred, out[i].Kind) < osRank(preferred, out[j].Kind)
	})
	return out
}

func hasKind(list []AssetOption, kind config.AssetKind) bool {
	for _, o := range list {
		if o.Kind == kind {
			return true
		}
	}
	return false
}

func osRank(preferred []config.AssetKind, kind config.AssetKind) int {
	for i, p := range preferred {
		if p == kind {
			return i
		}
	}
	return len(preferred) + 1
}

// OSDefaultKind — какой файл релиза подходит под текущую ОС.
func OSDefaultKind(slug string, rel *model.ReleaseInfo) config.AssetKind {
	main := MainProductPrefix(slug, rel.Tag)
	preferred := config.ForCurrentOS()
	for _, k := range preferred {
		for _, a := range rel.Assets {
			if kind, ok := AssetKindOf(a.Name); ok && kind == k && strings.HasPrefix(a.Name, main) {
				return k
			}
		}
	}
	for _, k := range preferred {
		for _, a := range rel.Assets {
			if kind, ok := AssetKindOf(a.Name); ok && kind == k {
				return k
			}
		}
	}
	return config.AssetSource
}
