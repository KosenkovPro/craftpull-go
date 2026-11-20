// Package storage — работа с версиями на диске: каталоги, сканирование, очистка.
package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/KosenkovPro/craftpull-go/internal/model"
)

// RepoName — имя каталога репозитория (без владельца).
func RepoName(slug string) string {
	if i := strings.LastIndex(slug, "/"); i >= 0 {
		return slug[i+1:]
	}
	return slug
}

// RepoDir — каталог репозитория внутри корня загрузок.
func RepoDir(root, slug string) string {
	return filepath.Join(root, RepoName(slug))
}

// VersionDir — каталог конкретной версии.
func VersionDir(root, slug, tag string) string {
	return filepath.Join(RepoDir(root, slug), Sanitize(tag))
}

// SourceDir — каталог для исходников репозитория (git-клон).
func SourceDir(root, slug string) string {
	return filepath.Join(root, "_sources", RepoName(slug))
}

// Sanitize приводит имя тега к безопасному виду для имени каталога.
func Sanitize(tag string) string {
	var b strings.Builder
	b.Grow(len(tag))
	for _, c := range tag {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
			b.WriteRune(c)
		case c == '-' || c == '.' || c == '_':
			b.WriteRune(c)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}

// VersionMeta — метаданные версии внутри каталога.
type VersionMeta struct {
	Tag         string `json:"tag"`
	Channel     string `json:"channel"`
	InstalledAt string `json:"installed_at"`
}

// WriteMeta сохраняет метаданные версии.
func WriteMeta(dir string, meta VersionMeta) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "meta.json"), data, 0o644)
}

// ReadMeta читает метаданные версии.
func ReadMeta(dir string) (VersionMeta, bool) {
	data, err := os.ReadFile(filepath.Join(dir, "meta.json"))
	if err != nil {
		return VersionMeta{}, false
	}
	var meta VersionMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return VersionMeta{}, false
	}
	return meta, true
}

// DirSize — размер каталога в байтах.
func DirSize(dir string) uint64 {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	var total uint64
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			total += DirSize(path)
			continue
		}
		if info, err := entry.Info(); err == nil {
			total += uint64(info.Size())
		}
	}
	return total
}

// ScanInstalled находит уже скачанные версии репозитория.
func ScanInstalled(root, slug string) []model.InstalledVersion {
	out := make([]model.InstalledVersion, 0, 8)
	dir := RepoDir(root, slug)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return out
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		meta, hasMeta := ReadMeta(path)
		tag := entry.Name()
		if hasMeta && meta.Tag != "" {
			tag = meta.Tag
		}
		files := make([]string, 0, 8)
		if inner, err := os.ReadDir(path); err == nil {
			for _, f := range inner {
				name := f.Name()
				// meta.json — служебный файл, а не содержимое релиза.
				if name == "meta.json" {
					continue
				}
				files = append(files, name)
			}
		}
		sort.Strings(files)
		// Каталог без файлов — неудачная загрузка, а не установленная версия.
		if len(files) == 0 {
			continue
		}
		version := model.InstalledVersion{
			Tag:   tag,
			Dir:   path,
			Size:  DirSize(path),
			Files: files,
		}
		if hasMeta {
			channel := meta.Channel
			installedAt := meta.InstalledAt
			version.Channel = &channel
			version.InstalledAt = &installedAt
		}
		out = append(out, version)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Tag > out[j].Tag })
	return out
}

// CleanupVersions удаляет все версии репозитория, кроме keep и закреплённых.
// Возвращает список удалённых каталогов.
func CleanupVersions(root, slug string, keep map[string]bool, pinned []string) []string {
	removed := make([]string, 0, 4)
	dir := RepoDir(root, slug)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return removed
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		name := entry.Name()
		tag := name
		if meta, ok := ReadMeta(path); ok && meta.Tag != "" {
			tag = meta.Tag
		}
		if keep[tag] || keep[name] {
			continue
		}
		pinnedHere := false
		for _, p := range pinned {
			if p == tag || p == name {
				pinnedHere = true
				break
			}
		}
		if pinnedHere {
			continue
		}
		if os.RemoveAll(path) == nil {
			removed = append(removed, path)
		}
	}
	return removed
}

// RemoveVersion удаляет конкретную версию (если она не закреплена).
func RemoveVersion(root, slug, tag string, pinned []string) bool {
	for _, p := range pinned {
		if p == tag {
			return false
		}
	}
	dir := VersionDir(root, slug, tag)
	if _, err := os.Stat(dir); err != nil {
		return false
	}
	return os.RemoveAll(dir) == nil
}
