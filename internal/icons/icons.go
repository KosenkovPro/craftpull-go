// Package icons — иконки репозиториев: скачивание с GitHub и локальный кэш.
//
// Иконки сторонних приложений никогда не попадают в репозиторий программы:
// это авторские права их владельцев. Кэш лежит в папке данных приложения.
package icons

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
)

// Disabled — иконки сторонних приложений отключены.
var disabled atomic.Bool

// SetDisabled включает или выключает показ иконок сторонних приложений.
func SetDisabled(v bool) { disabled.Store(v) }

// Disabled сообщает, отключены ли иконки.
func Disabled() bool { return disabled.Load() }

// Candidates — возможные пути к иконке внутри репозитория.
func Candidates(slug string) []string {
	name := slug
	if i := strings.LastIndex(slug, "/"); i >= 0 {
		name = slug[i+1:]
	}
	return []string{
		"assets/app-icon/hicolor/256x256/apps/ai.storyteller." + name + ".png",
		"assets/app-icon/hicolor/128x128/apps/ai.storyteller." + name + ".png",
		"assets/app-icon/hicolor/64x64/apps/ai.storyteller." + name + ".png",
		"assets/app-icon/hicolor/256x256/apps/" + name + ".png",
		"assets/icons/" + name + ".png",
		"assets/icon.png",
		"assets/app-icon.png",
		"icon.png",
	}
}

// CacheName — имя файла иконки в кэше.
func CacheName(slug string) string {
	return strings.ReplaceAll(slug, "/", "__") + ".png"
}

// EnsureIcon скачивает иконку репозитория и кладёт её в локальный кэш.
func EnsureIcon(ctx context.Context, client *http.Client, slug, dir string) (string, bool) {
	if Disabled() {
		return "", false
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", false
	}
	file := filepath.Join(dir, CacheName(slug))
	if info, err := os.Stat(file); err == nil && info.Size() > 0 {
		return file, true
	}
	for _, branch := range []string{"main", "master"} {
		for _, path := range Candidates(slug) {
			url := "https://raw.githubusercontent.com/" + slug + "/" + branch + "/" + path
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				continue
			}
			req.Header.Set("User-Agent", "CraftPull-Go/0.1")
			resp, err := client.Do(req)
			if err != nil {
				continue
			}
			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				resp.Body.Close()
				continue
			}
			data, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil || len(data) == 0 {
				continue
			}
			if err := os.WriteFile(file, data, 0o644); err != nil {
				continue
			}
			return file, true
		}
	}
	return "", false
}
