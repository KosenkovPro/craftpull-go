package selfupdate

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
)

// Fetch скачивает обновление и достаёт из архива готовый файл программы.
//
// Всё складывается в папку dir — обычно это <data>/update/<тег>: старые
// загрузки не мешают друг другу, а недокачанный файл не путается с новым.
//
// Возвращает путь к готовому файлу, который можно запускать.
func Fetch(ctx context.Context, client *http.Client, token string, u *Update, dir string,
	progress func(done, total uint64)) (string, error) {
	if u == nil {
		return "", os.ErrInvalid
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	archive := filepath.Join(dir, filepath.Base(u.Asset.Name))
	if err := Download(ctx, client, token, u.Asset, archive, progress); err != nil {
		return "", err
	}
	return ExtractPayload(archive, filepath.Join(dir, "payload"))
}

// UpdateDir — папка, куда складывается обновление указанной версии.
func UpdateDir(base, version string) string {
	return filepath.Join(base, "update", version)
}
