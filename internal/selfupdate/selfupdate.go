// Package selfupdate — автообновление программы.
//
// Механизм простой: программа спрашивает у GitHub последний релиз, выбирает
// файл под текущую систему, скачивает его, сверяет SHA-256 с тем digest,
// который отдаёт GitHub, и подменяет себя новой версией с перезапуском.
//
// Файлы Go-репозитория называются так:
// craftpull-windows-amd64.zip — Windows;
// craftpull-darwin-amd64, craftpull-darwin-arm64, craftpull-darwin-universal — macOS;
// craftpull-linux-amd64.tar.gz — Linux.
package selfupdate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

// UserAgent — как программа представляется GitHub.
const UserAgent = "CraftPull-Go"

// DefaultRepo — репозиторий с релизами самой программы.
//
// Пользователь может указать другой через настройку self_update_repo,
// но по умолчанию обновление идёт с официального репозитория.
const DefaultRepo = "KosenkovPro/craftpull-go"

// Timeout — разумное время ожидания ответа GitHub.
const Timeout = 30 * time.Second

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

// Update — найденное обновление.
type Update struct {
	// Version — тег новой версии, например v0.4.0.
	Version string
	// Asset — файл релиза под текущую систему.
	Asset Asset
}

// LatestRelease спрашивает о последнем релизе репозитория.
func LatestRelease(ctx context.Context, client *http.Client, token, slug string) (Release, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", slug)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Release{}, err
	}
	req.Header.Set("User-Agent", UserAgent)
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
		return Release{}, fmt.Errorf("GitHub вернул %s", resp.Status)
	}
	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return Release{}, err
	}
	return rel, nil
}

// Check ищет обновление: возвращает найденное или nil, если установлена
// свежая версия.
func Check(ctx context.Context, client *http.Client, token, slug, currentVersion string) (*Update, error) {
	rel, err := LatestRelease(ctx, client, token, slug)
	if err != nil {
		return nil, err
	}
	if !IsNewer(rel.TagName, currentVersion) {
		return nil, nil
	}
	asset, ok := PickAsset(rel)
	if !ok {
		return nil, fmt.Errorf("в релизе %s нет файла под %s/%s", rel.TagName, runtime.GOOS, runtime.GOARCH)
	}
	return &Update{Version: rel.TagName, Asset: asset}, nil
}

// CompareVersions сравнивает две версии: -1, 0 или 1.
func CompareVersions(a, b string) int {
	pa := versionParts(a)
	pb := versionParts(b)
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			x = pa[i]
		}
		if i < len(pb) {
			y = pb[i]
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}

// IsNewer сообщает, что версия candidate новее current.
func IsNewer(candidate, current string) bool {
	return CompareVersions(candidate, current) > 0
}

// versionParts разбирает версию на числовые части.
func versionParts(v string) []int {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	parts := strings.Split(v, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n := 0
		for _, r := range p {
			if r < '0' || r > '9' {
				break
			}
			n = n*10 + int(r-'0')
		}
		out = append(out, n)
	}
	return out
}

// archTokenFor переводит архитектуру Go в обозначение в именах файлов.
func archTokenFor(goarch string) string {
	switch goarch {
	case "amd64":
		return "amd64"
	case "arm64":
		return "arm64"
	default:
		return goarch
	}
}

// PickAsset выбирает файл релиза под текущую систему.
func PickAsset(rel Release) (Asset, bool) {
	return pickAsset(rel, runtime.GOOS, runtime.GOARCH)
}

// pickAsset выбирает файл релиза под указанную систему.
//
// Windows: craftpull-windows-amd64.zip;
// macOS: craftpull-darwin-amd64, craftpull-darwin-arm64, craftpull-darwin-universal;
// Linux: craftpull-linux-amd64.tar.gz.
func pickAsset(rel Release, goos, goarch string) (Asset, bool) {
	arch := archTokenFor(goarch)
	switch goos {
	case "windows":
		for _, a := range rel.Assets {
			n := strings.ToLower(a.Name)
			if strings.Contains(n, "windows") && strings.Contains(n, arch) {
				return a, true
			}
		}
	case "darwin":
		// Сначала готовый бинарник под архитектуру: его не нужно распаковывать.
		for _, a := range rel.Assets {
			n := strings.ToLower(a.Name)
			if strings.Contains(n, "darwin") && strings.Contains(n, arch) &&
				!strings.HasSuffix(n, ".zip") && !strings.HasSuffix(n, ".dmg") &&
				!strings.Contains(n, "legacy") {
				return a, true
			}
		}
		// Потом universal — он запускается на обеих архитектурах.
		for _, a := range rel.Assets {
			n := strings.ToLower(a.Name)
			if strings.Contains(n, "darwin") && strings.Contains(n, "universal") &&
				!strings.Contains(n, "legacy") {
				return a, true
			}
		}
	case "linux":
		for _, a := range rel.Assets {
			n := strings.ToLower(a.Name)
			if strings.Contains(n, "linux") && strings.Contains(n, arch) {
				return a, true
			}
		}
	}
	return Asset{}, false
}

// Download скачивает файл релиза и сверяет его SHA-256 с digest от GitHub.
//
// Если digest пуст, файл принимается без проверки — так бывает у старых
// релизов, загруженных до того, как GitHub начал отдавать контрольные суммы.
func Download(ctx context.Context, client *http.Client, token string, a Asset, dest string, progress func(done, total uint64)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.URL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", UserAgent)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("GitHub вернул %s", resp.Status)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	hash := sha256.New()
	written, err := io.Copy(out, io.TeeReader(resp.Body, hash))
	if err != nil {
		return err
	}
	if progress != nil {
		progress(uint64(written), a.Size)
	}

	want := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(a.Digest, "sha256:"), "SHA256:"))
	if want == "" {
		return nil
	}
	got := hex.EncodeToString(hash.Sum(nil))
	if !strings.EqualFold(want, got) {
		_ = os.Remove(dest)
		return fmt.Errorf("не совпала контрольная сумма: ожидалось %s, получено %s", want, got)
	}
	return nil
}

// copyFile копирует файл и сохраняет права на запуск.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
