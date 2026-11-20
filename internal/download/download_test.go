package download

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/KosenkovPro/craftpull-go/internal/model"
)

// fileServer отдаёт содержимое файла и поддерживает Range-запросы.
func fileServer(data []byte) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if rng := r.Header.Get("Range"); rng != "" {
			var start int
			if _, err := fmt.Sscanf(rng, "bytes=%d-", &start); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			if start >= len(data) {
				w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
				return
			}
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, len(data)-1, len(data)))
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write(data[start:])
			return
		}
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
		_, _ = w.Write(data)
	}))
}

func newDownloader(t *testing.T) *Downloader {
	t.Helper()
	return &Downloader{
		Client:       http.DefaultClient,
		Limiter:      NewRateLimiter(),
		Control:      NewControl(),
		Tasks:        NewTaskRegistry(),
		VerifySHA256: true,
	}
}

// TestFetch проверяет обычную загрузку файла.
func TestFetch(t *testing.T) {
	payload := make([]byte, 200*1024)
	for i := range payload {
		payload[i] = byte(i % 251)
	}
	srv := fileServer(payload)
	defer srv.Close()

	sum := sha256.Sum256(payload)
	digest := "sha256:" + hex.EncodeToString(sum[:])

	asset := model.AssetInfo{
		Name:   "artifact.bin",
		URL:    srv.URL + "/artifact.bin",
		Size:   uint64(len(payload)),
		Digest: &digest,
	}
	dir := t.TempDir()
	d := newDownloader(t)

	if err := d.Fetch(context.Background(), "task-1", "owner/repo", "v1", asset, dir); err != nil {
		t.Fatalf("загрузка не удалась: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, "artifact.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(payload) {
		t.Fatalf("скачано %d байт, ожидалось %d", len(got), len(payload))
	}
	if _, err := os.Stat(filepath.Join(dir, "artifact.bin.part")); err == nil {
		t.Fatal("временный файл .part не удалён")
	}
}

// TestFetchResume проверяет докачку: сервер отдаёт только вторую половину.
func TestFetchResume(t *testing.T) {
	payload := make([]byte, 150*1024)
	for i := range payload {
		payload[i] = byte(i % 97)
	}
	srv := fileServer(payload)
	defer srv.Close()

	asset := model.AssetInfo{
		Name: "artifact.bin",
		URL:  srv.URL + "/artifact.bin",
		Size: uint64(len(payload)),
	}
	dir := t.TempDir()

	// Первая половина уже скачана.
	part := filepath.Join(dir, "artifact.bin.part")
	if err := os.WriteFile(part, payload[:len(payload)/2], 0o644); err != nil {
		t.Fatal(err)
	}

	d := newDownloader(t)
	if err := d.Fetch(context.Background(), "task-2", "owner/repo", "v1", asset, dir); err != nil {
		t.Fatalf("докачка не удалась: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, "artifact.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(payload) {
		t.Fatalf("после докачки %d байт, ожидалось %d", len(got), len(payload))
	}
	for i := range got {
		if got[i] != payload[i] {
			t.Fatalf("байт %d не совпадает после докачки", i)
		}
	}
}

// TestFetchChecksumMismatch проверяет, что неверная контрольная сумма
// останавливает загрузку.
func TestFetchChecksumMismatch(t *testing.T) {
	payload := []byte("craftpull")
	srv := fileServer(payload)
	defer srv.Close()

	wrong := "sha256:" + hex.EncodeToString(make([]byte, 32))
	asset := model.AssetInfo{
		Name:   "artifact.bin",
		URL:    srv.URL + "/artifact.bin",
		Size:   uint64(len(payload)),
		Digest: &wrong,
	}
	dir := t.TempDir()
	d := newDownloader(t)

	if err := d.Fetch(context.Background(), "task-3", "owner/repo", "v1", asset, dir); err == nil {
		t.Fatal("ожидалась ошибка контрольной суммы")
	}
	if _, err := os.Stat(filepath.Join(dir, "artifact.bin")); err == nil {
		t.Fatal("битый файл не удалён")
	}
}
