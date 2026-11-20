// Package download — загрузка файлов: лимит скорости, пауза с докачкой,
// отмена, проверка SHA-256.
package download

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/KosenkovPro/craftpull-go/internal/i18n"
	"github.com/KosenkovPro/craftpull-go/internal/model"
)

// TaskRegistry — реестр активных задач: id → прогресс.
type TaskRegistry struct {
	mu    sync.RWMutex
	tasks map[string]*model.TaskProgress
}

// NewTaskRegistry создаёт пустой реестр.
func NewTaskRegistry() *TaskRegistry {
	return &TaskRegistry{tasks: make(map[string]*model.TaskProgress)}
}

// Set обновляет или создаёт запись о задаче.
func (r *TaskRegistry) Set(id string, fn func(t *model.TaskProgress)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	t, ok := r.tasks[id]
	if !ok {
		t = &model.TaskProgress{ID: id}
		r.tasks[id] = t
	}
	fn(t)
}

// Get возвращает копию прогресса задачи.
func (r *TaskRegistry) Get(id string) (model.TaskProgress, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tasks[id]
	if !ok {
		return model.TaskProgress{}, false
	}
	return *t, true
}

// All возвращает копии всех задач.
func (r *TaskRegistry) All() []model.TaskProgress {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]model.TaskProgress, 0, len(r.tasks))
	for _, t := range r.tasks {
		out = append(out, *t)
	}
	return out
}

// Remove убирает задачу из реестра.
func (r *TaskRegistry) Remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tasks, id)
}

// Control — глобальные команды пользователя: пауза, остановка.
type Control struct {
	mu        sync.Mutex
	paused    bool
	cancelled bool
	done      chan struct{}
	closeOnce sync.Once
}

// NewControl создаёт контроллер команд.
func NewControl() *Control {
	return &Control{done: make(chan struct{})}
}

// TogglePause переключает паузу и возвращает новое состояние.
func (c *Control) TogglePause() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.paused = !c.paused
	return c.paused
}

// IsPaused сообщает, стоит ли загрузка на паузе.
func (c *Control) IsPaused() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.paused
}

// Cancel останавливает все загрузки: закрывается канал done.
func (c *Control) Cancel() {
	c.mu.Lock()
	c.cancelled = true
	c.mu.Unlock()
	c.closeOnce.Do(func() { close(c.done) })
}

// Reset снимает отмену — новый канал для следующей серии загрузок.
func (c *Control) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cancelled {
		c.cancelled = false
		c.closeOnce = sync.Once{}
		c.done = make(chan struct{})
	}
}

// IsCancelled сообщает, запрошена ли остановка.
func (c *Control) IsCancelled() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cancelled
}

// Done возвращает канал, закрываемый при остановке загрузок.
func (c *Control) Done() <-chan struct{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.done
}

// RateLimiter — token-bucket: ограничение общей скорости по всем загрузкам.
type RateLimiter struct {
	limit  atomic.Uint64
	mu     sync.Mutex
	tokens float64
	last   time.Time
}

// NewRateLimiter создаёт ограничитель без лимита.
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{last: time.Now()}
}

// SetLimit задаёт лимит в байтах в секунду; 0 — без ограничения.
func (r *RateLimiter) SetLimit(bytesPerSec uint64) { r.limit.Store(bytesPerSec) }

// Limit возвращает текущий лимит.
func (r *RateLimiter) Limit() uint64 { return r.limit.Load() }

// Acquire забирает из квоты до want байт. Если квоты не хватает — ждёт и
// возвращает, сколько удалось получить сейчас.
func (r *RateLimiter) Acquire(ctx context.Context, want uint64) uint64 {
	limit := r.Limit()
	if limit == 0 || want == 0 {
		return want
	}
	r.mu.Lock()
	now := time.Now()
	dt := now.Sub(r.last).Seconds()
	r.last = now
	r.tokens = minFloat(r.tokens+dt*float64(limit), float64(limit))
	grant := uint64(minFloat(r.tokens, float64(want)))
	r.tokens -= float64(grant)
	r.mu.Unlock()

	if grant < want {
		deficit := float64(want - grant)
		wait := deficit / float64(limit)
		if wait < 0.02 {
			wait = 0.02
		}
		if wait > 2.0 {
			wait = 2.0
		}
		timer := time.NewTimer(time.Duration(wait * float64(time.Second)))
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return grant
		case <-timer.C:
		}
	}
	return grant
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// Downloader — загрузчик одного ассета.
type Downloader struct {
	Client       *http.Client
	Limiter      *RateLimiter
	Control      *Control
	Tasks        *TaskRegistry
	VerifySHA256 bool
	// Seq — порядковый номер задачи в очереди загрузки.
	Seq uint64
}

// Fetch скачивает ассет в каталог destDir.
//
// Возвращает nil при успехе, текст ошибки при ошибке или отмене.
func (d *Downloader) Fetch(ctx context.Context, id, repo, tag string, asset model.AssetInfo, destDir string) error {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return errors.New(i18n.TF1("err.mkdir", "{e}", err.Error()))
	}

	dest := filepath.Join(destDir, asset.Name)
	part := filepath.Join(destDir, asset.Name+".part")

	// Докачка: если .part уже есть — продолжаем с его размера.
	var downloaded uint64
	if info, err := os.Stat(part); err == nil {
		downloaded = uint64(info.Size())
	}
	if downloaded > 0 && asset.Size > 0 && downloaded >= asset.Size {
		// Файл уже скачан целиком.
		if err := os.Rename(part, dest); err != nil {
			return errors.New(i18n.TF1("err.save_file", "{e}", err.Error()))
		}
		d.finish(id, model.TaskDone, downloaded, asset.Size, nil)
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.URL, nil)
	if err != nil {
		msg := i18n.TF1("err.download", "{e}", err.Error())
		d.finish(id, model.TaskFailed, downloaded, asset.Size, &msg)
		return fmt.Errorf("%s", msg)
	}
	req.Header.Set("User-Agent", "CraftPull-Go/0.1")
	if downloaded > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", downloaded))
	}

	resp, err := d.Client.Do(req)
	if err != nil {
		msg := i18n.TF1("err.download", "{e}", err.Error())
		d.finish(id, model.TaskFailed, downloaded, asset.Size, &msg)
		return fmt.Errorf("%s", msg)
	}
	defer resp.Body.Close()

	appendMode := false
	switch {
	case resp.StatusCode == http.StatusPartialContent:
		appendMode = true
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		// Сервер не поддержал Range — начинаем заново.
		downloaded = 0
	default:
		msg := fmt.Sprintf("HTTP %s", resp.Status)
		d.finish(id, model.TaskFailed, downloaded, asset.Size, &msg)
		return fmt.Errorf("%s", msg)
	}

	total := asset.Size
	if resp.ContentLength > 0 {
		total = uint64(resp.ContentLength) + downloaded
	} else if total < downloaded {
		total = downloaded
	}

	flags := os.O_CREATE | os.O_WRONLY
	if appendMode {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}
	file, err := os.OpenFile(part, flags, 0o644)
	if err != nil {
		msg := i18n.TF1("err.open_file", "{e}", err.Error())
		d.finish(id, model.TaskFailed, downloaded, total, &msg)
		return fmt.Errorf("%s", msg)
	}

	d.update(id, repo, tag, asset, dest, total, downloaded, 0, model.TaskRunning, nil)

	buf := make([]byte, 32*1024)
	lastReport := time.Now()
	lastBytes := downloaded
	speed := 0.0

	for {
		// Пауза: перестаём читать поток, пока пользователь не продолжит.
		for d.Control.IsPaused() {
			if d.Control.IsCancelled() {
				file.Close()
				d.finish(id, model.TaskCancelled, downloaded, total, nil)
				return fmt.Errorf("%s", i18n.T("err.stopped"))
			}
			d.update(id, repo, tag, asset, dest, total, downloaded, speed, model.TaskPaused, nil)
			select {
			case <-ctx.Done():
				file.Close()
				d.finish(id, model.TaskCancelled, downloaded, total, nil)
				return fmt.Errorf("%s", i18n.T("err.stopped"))
			case <-d.Control.Done():
				file.Close()
				d.finish(id, model.TaskCancelled, downloaded, total, nil)
				return fmt.Errorf("%s", i18n.T("err.stopped"))
			case <-time.After(120 * time.Millisecond):
			}
		}

		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			off := 0
			for off < n {
				want := uint64(n - off)
				granted := d.acquire(ctx, want)
				if granted == 0 {
					if d.Control.IsCancelled() {
						file.Close()
						d.finish(id, model.TaskCancelled, downloaded, total, nil)
						return fmt.Errorf("%s", i18n.T("err.stopped"))
					}
					time.Sleep(50 * time.Millisecond)
					continue
				}
				if granted > want {
					granted = want
				}
				written, werr := file.Write(buf[off : off+int(granted)])
				if werr != nil {
					msg := i18n.TF1("err.write", "{e}", werr.Error())
					file.Close()
					d.finish(id, model.TaskFailed, downloaded, total, &msg)
					return fmt.Errorf("%s", msg)
				}
				off += int(granted)
				downloaded += uint64(written)
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			msg := i18n.TF1("err.conn_drop", "{e}", readErr.Error())
			file.Close()
			d.finish(id, model.TaskFailed, downloaded, total, &msg)
			return fmt.Errorf("%s", msg)
		}

		now := time.Now()
		if now.Sub(lastReport) >= 400*time.Millisecond {
			dt := now.Sub(lastReport).Seconds()
			if dt < 0.001 {
				dt = 0.001
			}
			instant := float64(downloaded-lastBytes) / dt
			// Скорость сглаживаем: мгновенная скачет, и оценка остатка мигает.
			if speed <= 0 {
				speed = instant
			} else {
				speed = speed*0.6 + instant*0.4
			}
			lastReport = now
			lastBytes = downloaded
			d.update(id, repo, tag, asset, dest, total, downloaded, speed, model.TaskRunning, nil)
		}
	}

	if err := file.Sync(); err != nil {
		msg := i18n.TF1("err.save", "{e}", err.Error())
		file.Close()
		d.finish(id, model.TaskFailed, downloaded, total, &msg)
		return fmt.Errorf("%s", msg)
	}
	file.Close()

	if err := os.Rename(part, dest); err != nil {
		msg := i18n.TF1("err.rename", "{e}", err.Error())
		d.finish(id, model.TaskFailed, downloaded, total, &msg)
		return fmt.Errorf("%s", msg)
	}

	// Проверка контрольной суммы, если GitHub её отдаёт.
	if d.VerifySHA256 && asset.Digest != nil {
		expected := strings.TrimPrefix(*asset.Digest, "sha256:")
		if expected != "" {
			actual, err := SHA256File(dest)
			if err != nil {
				msg := i18n.TF1("err.sha_read", "{e}", err.Error())
				d.finish(id, model.TaskFailed, downloaded, total, &msg)
				return fmt.Errorf("%s", msg)
			}
			if !strings.EqualFold(actual, expected) {
				_ = os.Remove(dest)
				msg := i18n.TF("err.sha_mismatch", map[string]string{
					"{expected}": truncate(expected, 16),
					"{actual}":   actual,
				})
				d.finish(id, model.TaskFailed, downloaded, total, &msg)
				return fmt.Errorf("%s", msg)
			}
		}
	}

	d.finish(id, model.TaskDone, downloaded, total, nil)
	return nil
}

// acquire забирает квоту скорости, прерывая ожидание при отмене.
func (d *Downloader) acquire(ctx context.Context, want uint64) uint64 {
	if d.Limiter == nil {
		return want
	}
	type result struct{ n uint64 }
	ch := make(chan result, 1)
	go func() { ch <- result{d.Limiter.Acquire(ctx, want)} }()
	select {
	case r := <-ch:
		return r.n
	case <-d.Control.Done():
		return 0
	case <-ctx.Done():
		return 0
	}
}

func (d *Downloader) update(id, repo, tag string, asset model.AssetInfo, dest string,
	total, downloaded uint64, speed float64, state model.TaskState, errMsg *string) {
	d.Tasks.Set(id, func(t *model.TaskProgress) {
		t.ID = id
		t.Repo = repo
		t.Tag = tag
		t.Asset = asset.Name
		t.Dest = dest
		t.Total = total
		t.Downloaded = downloaded
		t.Speed = speed
		t.State = state
		t.Error = errMsg
		t.Seq = d.Seq
		t.StartedAt = model.NowMS()
	})
}

func (d *Downloader) finish(id string, state model.TaskState, downloaded, total uint64, errMsg *string) {
	d.Tasks.Set(id, func(t *model.TaskProgress) {
		t.Downloaded = downloaded
		t.Total = total
		t.State = state
		t.Error = errMsg
		t.Speed = 0
	})
}

// SHA256File считает SHA-256 файла.
func SHA256File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// TaskID — уникальный идентификатор задачи.
func TaskID(repo, tag, asset string) string {
	return fmt.Sprintf("%s::%s::%s", repo, tag, asset)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
