// Package model — общая модель данных: релизы, ассеты, состояние задач.
package model

import (
	"fmt"
	"time"

	"github.com/KosenkovPro/craftpull-go/internal/i18n"
)

// Channel — канал распространения сборки.
type Channel int

const (
	// ChannelStable — стабильный релиз.
	ChannelStable Channel = iota
	// ChannelEdge — нестабильный: pre-release / nightly / rc.
	ChannelEdge
)

// Label возвращает подпись канала с учётом языка интерфейса.
func (c Channel) Label() string {
	if c == ChannelEdge {
		return i18n.T("popup.edge")
	}
	return i18n.T("popup.stable")
}

// AssetInfo — один файл внутри релиза GitHub.
type AssetInfo struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Size uint64 `json:"size"`
	// Digest — sha256:<hex>, если GitHub его отдаёт.
	Digest *string `json:"digest"`
	// Synthetic — ассет собран программой (архив исходников).
	Synthetic bool `json:"synthetic"`
}

// SourceZip создаёт синтетический ассет — архив исходников репозитория.
func SourceZip(tag, url string) AssetInfo {
	return AssetInfo{
		Name:      fmt.Sprintf("%s-source.zip", tag),
		URL:       url,
		Size:      0,
		Digest:    nil,
		Synthetic: true,
	}
}

// ReleaseInfo — релиз репозитория.
type ReleaseInfo struct {
	Tag         string      `json:"tag"`
	Name        string      `json:"name"`
	Prerelease  bool        `json:"prerelease"`
	Draft       bool        `json:"draft"`
	PublishedAt *string     `json:"published_at"`
	HTMLURL     string      `json:"html_url"`
	Assets      []AssetInfo `json:"assets"`
	SourceZip   string      `json:"source_zip"`
}

// IsEdge — сборка считается нестабильной, если GitHub помечает релиз как
// pre-release либо в теге есть ночные/предрелизные маркеры.
func (r *ReleaseInfo) IsEdge() bool {
	if r.Prerelease {
		return true
	}
	tag := lower(r.Tag)
	for _, m := range []string{"nightly", "dev", "alpha", "beta", "rc", "-pre", "preview"} {
		if contains(tag, m) {
			return true
		}
	}
	return false
}

// Channel возвращает канал релиза.
func (r *ReleaseInfo) Channel() Channel {
	if r.IsEdge() {
		return ChannelEdge
	}
	return ChannelStable
}

// TaskState — состояние одной загрузки.
type TaskState int

const (
	// TaskQueued — задача в очереди.
	TaskQueued TaskState = iota
	// TaskRunning — идёт скачивание.
	TaskRunning
	// TaskPaused — загрузка приостановлена.
	TaskPaused
	// TaskDone — файл скачан.
	TaskDone
	// TaskFailed — ошибка.
	TaskFailed
	// TaskCancelled — остановлено пользователем.
	TaskCancelled
)

// IsFinished — задача больше не активна.
func (s TaskState) IsFinished() bool {
	return s == TaskDone || s == TaskFailed || s == TaskCancelled
}

// Label — подпись состояния с учётом языка интерфейса.
func (s TaskState) Label() string {
	switch s {
	case TaskQueued:
		return i18n.T("downloads.state_queued")
	case TaskRunning:
		return i18n.T("downloads.state_running")
	case TaskPaused:
		return i18n.T("downloads.state_paused")
	case TaskDone:
		return i18n.T("downloads.state_done")
	case TaskFailed:
		return i18n.T("downloads.state_failed")
	case TaskCancelled:
		return i18n.T("downloads.state_cancelled")
	}
	return ""
}

// TaskProgress — прогресс одной загрузки.
type TaskProgress struct {
	ID         string `json:"id"`
	Repo       string `json:"repo"`
	Tag        string `json:"tag"`
	Asset      string `json:"asset"`
	Dest       string `json:"dest"`
	Total      uint64 `json:"total"`
	Downloaded uint64 `json:"downloaded"`
	// Speed — текущая скорость, байт/с.
	Speed float64   `json:"speed"`
	State TaskState `json:"state"`
	Error *string   `json:"error"`
	// Seq — порядковый номер постановки в очередь: чем меньше, тем раньше.
	Seq uint64 `json:"seq"`
	// StartedAt — момент постановки в очередь, мс с начала эпохи.
	StartedAt uint64 `json:"started_at"`
	// Queue — место в очереди загрузки (1 — первый).
	Queue int `json:"queue"`
	// QueueTotal — сколько всего задач в очереди.
	QueueTotal int `json:"queue_total"`
}

// Fraction — доля скачанного, 0..1.
func (t *TaskProgress) Fraction() float32 {
	if t.Total == 0 {
		if t.State == TaskDone {
			return 1
		}
		return 0
	}
	f := float32(t.Downloaded) / float32(t.Total)
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}

// ElapsedSecs — сколько секунд идёт загрузка.
func (t *TaskProgress) ElapsedSecs() uint64 {
	now := NowMS()
	if now <= t.StartedAt {
		return 0
	}
	return (now - t.StartedAt) / 1000
}

// RemainingSecs — осталось примерно столько секунд; ok=false, если оценки нет.
func (t *TaskProgress) RemainingSecs() (uint64, bool) {
	if t.Speed <= 1 || t.Total == 0 {
		return 0, false
	}
	left := uint64(0)
	if t.Total > t.Downloaded {
		left = t.Total - t.Downloaded
	}
	return uint64(float64(left) / t.Speed), true
}

// InstalledVersion — уже скачанная версия, найденная на диске.
type InstalledVersion struct {
	Tag         string   `json:"tag"`
	Dir         string   `json:"dir"`
	Size        uint64   `json:"size"`
	Files       []string `json:"files"`
	Channel     *string  `json:"channel"`
	InstalledAt *string  `json:"installed_at"`
}

// LogLevel — уровень записи в журнал.
type LogLevel int

const (
	// LogInfo — обычное сообщение.
	LogInfo LogLevel = iota
	// LogOk — успех.
	LogOk
	// LogWarn — предупреждение.
	LogWarn
	// LogError — ошибка.
	LogError
)

// LogEntry — запись журнала.
type LogEntry struct {
	Time  string   `json:"time"`
	Level LogLevel `json:"level"`
	Text  string   `json:"text"`
}

// NowMS — текущее время, мс с начала эпохи.
func NowMS() uint64 { return uint64(time.Now().UnixMilli()) }

// HumanDuration форматирует время в формате «мм:сс» или «ч:мм:сс».
func HumanDuration(secs uint64) string {
	h := secs / 3600
	m := (secs % 3600) / 60
	s := secs % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

// HumanDate форматирует дату релиза: RFC3339 → «12 Nov 2026, 02:18».
func HumanDate(raw string) string {
	if raw == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return raw
	}
	return t.Local().Format("02 Jan 2006, 15:04")
}

// HumanSize форматирует размер для интерфейса.
func HumanSize(bytes uint64) string {
	const (
		kb = 1024.0
		mb = kb * 1024.0
		gb = mb * 1024.0
	)
	b := float64(bytes)
	switch {
	case b >= gb:
		return i18n.TF1("unit.gb", "{v}", fmt.Sprintf("%.2f", b/gb))
	case b >= mb:
		return i18n.TF1("unit.mb", "{v}", fmt.Sprintf("%.1f", b/mb))
	case b >= kb:
		return i18n.TF1("unit.kb", "{v}", fmt.Sprintf("%.0f", b/kb))
	default:
		return i18n.TF1("unit.b", "{v}", fmt.Sprintf("%d", bytes))
	}
}

// HumanSpeed форматирует скорость.
func HumanSpeed(bytesPerSec float64) string {
	if bytesPerSec <= 0 {
		return "—"
	}
	return i18n.TF1("unit.per_sec", "{v}", HumanSize(uint64(bytesPerSec)))
}

func lower(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}

func contains(haystack, needle string) bool {
	if len(needle) == 0 {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
