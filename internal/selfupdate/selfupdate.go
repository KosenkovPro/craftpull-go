// Package selfupdate — самообновление программы.
//
// В отличие от Rust-версии, Go-версия не пересобирает себя из исходников:
// она скачивает готовый бинарник последнего релиза с GitHub и подменяет его
// при следующем запуске. Это работает одинаково на Windows, macOS и Linux.
package selfupdate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Asset — файл релиза.
type Asset struct {
	Name   string `json:"name"`
	URL    string `json:"browser_download_url"`
	Size   uint64 `json:"size"`
	Digest string `json:"digest"`
}

// Release — релиз GitHub.
type Release struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
}

// Update — подготовленное обновление.
type Update struct {
	Version string
	Asset   Asset
	Path    string
}

// osSuffix возвращает суффикс имён файлов для текущей ОС.
func osSuffix() string {
	switch runtime.GOOS {
	case "windows":
		return "windows"
	case "darwin":
		return "macos"
	default:
		return "linux"
	}
}

// archSuffix возвращает суффикс имён файлов для текущей архитектуры.
func archSuffix() string {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64"
	case "arm64":
		return "arm64"
	default:
		return runtime.GOARCH
	}
}

// LatestRelease спрашивает о последнем релизе репозитория.
func LatestRelease(ctx context.Context, client *http.Client, token, slug string) (Release, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", slug)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("User-Agent", "CraftPull-Go/0.1")
	req.Header.Set("Accept", "application/vnd.github+json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return Release{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Release{}, fmt.Errorf("GitHub returned %s", resp.Status)
	}
	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return Release{}, err
	}
	return rel, nil
}

// PickAsset выбирает файл релиза под текущую систему.
func PickAsset(rel Release) (Asset, bool) {
	wantOS := osSuffix()
	wantArch := archSuffix()
	var fallback Asset
	haveFallback := false
	for _, a := range rel.Assets {
		name := strings.ToLower(a.Name)
		if !strings.Contains(name, wantOS) && !strings.Contains(name, osSuffix()) {
			continue
		}
		if !strings.Contains(name, wantArch) && !strings.Contains(name, "universal") &&
			!strings.Contains(name, "all") {
			continue
		}
		if strings.Contains(name, wantArch) {
			return a, true
		}
		if !haveFallback {
			fallback = a
			haveFallback = true
		}
	}
	return fallback, haveFallback
}

// Prepare скачивает новую версию рядом с текущей программой и готовит замену.
//
// Возвращает путь к скачанному файлу — подмена происходит при следующем запуске.
func Prepare(ctx context.Context, client *http.Client, token, slug, currentVersion string) (Update, error) {
	rel, err := LatestRelease(ctx, client, token, slug)
	if err != nil {
		return Update{}, err
	}
	if currentVersion != "" && strings.TrimPrefix(rel.TagName, "v") == currentVersion {
		return Update{}, fmt.Errorf("уже установлена свежая версия %s", rel.TagName)
	}
	asset, ok := PickAsset(rel)
	if !ok {
		return Update{}, fmt.Errorf("в релизе %s нет файла под %s/%s", rel.TagName, osSuffix(), archSuffix())
	}

	exe, err := os.Executable()
	if err != nil {
		return Update{}, err
	}
	dir := filepath.Dir(exe)
	target := filepath.Join(dir, fmt.Sprintf("craftpull-%s%s", rel.TagName, filepath.Ext(asset.Name)))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.URL, nil)
	if err != nil {
		return Update{}, err
	}
	req.Header.Set("User-Agent", "CraftPull-Go/0.1")
	resp, err := client.Do(req)
	if err != nil {
		return Update{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Update{}, fmt.Errorf("GitHub returned %s", resp.Status)
	}
	out, err := os.Create(target)
	if err != nil {
		return Update{}, err
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		return Update{}, err
	}
	out.Close()
	if runtime.GOOS != "windows" {
		_ = os.Chmod(target, 0o755)
	}
	return Update{Version: rel.TagName, Asset: asset, Path: target}, nil
}

// Stage готовит замену текущего бинарника: новый файл переименовывается
// поверх старого при следующем запуске.
func Stage(currentPath, newPath string) error {
	if currentPath == "" || newPath == "" {
		return fmt.Errorf("пути не заданы")
	}
	if _, err := os.Stat(newPath); err != nil {
		return err
	}
	backup := currentPath + ".old"
	_ = os.Remove(backup)
	if err := os.Rename(currentPath, backup); err != nil {
		return err
	}
	if err := os.Rename(newPath, currentPath); err != nil {
		// Возвращаем как было — программа не должна остаться без файла.
		_ = os.Rename(backup, currentPath)
		return err
	}
	if runtime.GOOS != "windows" {
		_ = os.Chmod(currentPath, 0o755)
	}
	_ = os.Remove(backup)
	return nil
}

// PendingUpdate ищет скачанный файл обновления рядом с программой.
func PendingUpdate(currentPath string) (string, bool) {
	dir := filepath.Dir(currentPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "craftpull-v") {
			continue
		}
		full := filepath.Join(dir, name)
		if info, err := e.Info(); err == nil && info.Size() > 0 {
			return full, true
		}
	}
	return "", false
}

// Timeout — разумное время ожидания ответа GitHub.
const Timeout = 30 * time.Second
