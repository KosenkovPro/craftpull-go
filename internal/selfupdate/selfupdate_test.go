package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/KosenkovPro/craftpull-go/internal/model"
)

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"v0.4.0", "v0.3.0", 1},
		{"v0.3.0", "v0.4.0", -1},
		{"v0.4.0", "v0.4.0", 0},
		{"0.4.0", "v0.4.0", 0},
		{"v0.5", "v0.4.9", 1},
		{"v1.0.0", "v0.99.99", 1},
		{"v0.4.0-rc1", "v0.4.0", 0},
	}
	for _, c := range cases {
		if got := CompareVersions(c.a, c.b); got != c.want {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestIsNewer(t *testing.T) {
	if !IsNewer("v0.4.0", "v0.3.0") {
		t.Error("v0.4.0 should be newer than v0.3.0")
	}
	if IsNewer("v0.3.0", "v0.3.0") {
		t.Error("equal versions are not newer")
	}
	if IsNewer("v0.2.0", "v0.3.0") {
		t.Error("older version is not newer")
	}
}

// releaseFixture — набор файлов релиза, похожий на настоящий.
func releaseFixture() Release {
	return Release{
		TagName: "v0.4.0",
		Assets: []Asset{
			{Name: "craftpull-windows-amd64.zip", Size: 10},
			{Name: "craftpull-darwin-amd64", Size: 10},
			{Name: "craftpull-darwin-arm64", Size: 10},
			{Name: "craftpull-darwin-universal", Size: 10},
			{Name: "craftpull-darwin-universal-legacy", Size: 10},
			{Name: "craftpull-linux-amd64.tar.gz", Size: 10},
			{Name: "craftpull-macos-universal.dmg", Size: 10},
		},
	}
}

func TestPickAsset(t *testing.T) {
	rel := releaseFixture()
	cases := []struct {
		goos, goarch, want string
	}{
		{"windows", "amd64", "craftpull-windows-amd64.zip"},
		{"darwin", "amd64", "craftpull-darwin-amd64"},
		{"darwin", "arm64", "craftpull-darwin-arm64"},
		{"linux", "amd64", "craftpull-linux-amd64.tar.gz"},
	}
	for _, c := range cases {
		a, ok := pickAsset(rel, c.goos, c.goarch)
		if !ok {
			t.Errorf("pickAsset(%s/%s): no asset found", c.goos, c.goarch)
			continue
		}
		if a.Name != c.want {
			t.Errorf("pickAsset(%s/%s) = %q, want %q", c.goos, c.goarch, a.Name, c.want)
		}
	}
}

// На macOS без подходящей архитектуры берётся universal-файл,
// а legacy-сборки в выбор не попадают.
func TestPickAssetDarwinUniversal(t *testing.T) {
	rel := Release{TagName: "v0.4.0", Assets: []Asset{
		{Name: "craftpull-darwin-amd64-legacy"},
		{Name: "craftpull-darwin-universal"},
	}}
	a, ok := pickAsset(rel, "darwin", "arm64")
	if !ok || a.Name != "craftpull-darwin-universal" {
		t.Fatalf("pickAsset(darwin/arm64) = %q, %v", a.Name, ok)
	}
}

func TestPickAssetNotFound(t *testing.T) {
	rel := Release{TagName: "v0.4.0"}
	if _, ok := pickAsset(rel, "windows", "amd64"); ok {
		t.Error("empty release must not yield an asset")
	}
}

// writeZip собирает zip-архив с одним файлом.
func writeZip(t *testing.T, path, entry, content string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	w := zip.NewWriter(f)
	e, err := w.Create(entry)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

// writeTarGz собирает tar.gz-архив с одним файлом.
func writeTarGz(t *testing.T, path, entry, content string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{
		Name: entry, Mode: 0o755, Size: int64(len(content)), Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestExtractZip(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "craftpull-windows-amd64.zip")
	writeZip(t, archive, "craftpull-windows-amd64.exe", "binary")

	out, err := ExtractPayload(archive, filepath.Join(dir, "payload"))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "binary" {
		t.Errorf("payload = %q, want %q", data, "binary")
	}
}

// В zip с пакетом .app берётся файл внутри Contents/MacOS.
func TestExtractZipAppBundle(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "craftpull-macos-universal.zip")
	writeZip(t, archive, "CraftPull.app/Contents/MacOS/craftpull", "mac")

	out, err := ExtractPayload(archive, filepath.Join(dir, "payload"))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(out) != "craftpull" {
		t.Errorf("payload = %q, want craftpull", filepath.Base(out))
	}
}

func TestExtractTarGz(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "craftpull-linux-amd64.tar.gz")
	writeTarGz(t, archive, "craftpull", "linux")

	out, err := ExtractPayload(archive, filepath.Join(dir, "payload"))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "linux" {
		t.Errorf("payload = %q, want %q", data, "linux")
	}
}

// Готовый файл без расширения копируется как есть.
func TestExtractPlainBinary(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "craftpull-darwin-arm64")
	if err := os.WriteFile(src, []byte("bin"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, err := ExtractPayload(src, filepath.Join(dir, "payload"))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "bin" {
		t.Errorf("payload = %q, want %q", data, "bin")
	}
}

func TestExtractWithoutExecutable(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "craftpull-windows-amd64.zip")
	writeZip(t, archive, "README.md", "text")

	if _, err := ExtractPayload(archive, filepath.Join(dir, "payload")); err == nil {
		t.Error("archive without an executable must fail")
	}
}

// digestOf считает контрольную сумму файла в формате GitHub.
func digestOf(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func TestFetchVerifiesDigest(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "craftpull-windows-amd64.zip")
	writeZip(t, archive, "craftpull-windows-amd64.exe", "binary")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, archive)
	}))
	defer srv.Close()

	upd := &Update{Version: "v0.4.0", Asset: Asset{
		Name:   "craftpull-windows-amd64.zip",
		URL:    srv.URL + "/craftpull-windows-amd64.zip",
		Size:   10,
		Digest: digestOf(t, archive),
	}}
	payload, err := Fetch(context.Background(), srv.Client(), "", upd, filepath.Join(dir, "update"), nil)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(payload)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "binary" {
		t.Errorf("payload = %q, want %q", data, "binary")
	}
}

// Несовпадение контрольной суммы — файл не принимается и удаляется.
func TestFetchRejectsWrongDigest(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "craftpull-windows-amd64.zip")
	writeZip(t, archive, "craftpull-windows-amd64.exe", "binary")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, archive)
	}))
	defer srv.Close()

	upd := &Update{Version: "v0.4.0", Asset: Asset{
		Name:   "craftpull-windows-amd64.zip",
		URL:    srv.URL + "/craftpull-windows-amd64.zip",
		Digest: "sha256:" + hex.EncodeToString(make([]byte, 32)),
	}}
	target := filepath.Join(dir, "update")
	if _, err := Fetch(context.Background(), srv.Client(), "", upd, target, nil); err == nil {
		t.Fatal("wrong digest must be rejected")
	}
	entries, err := os.ReadDir(filepath.Join(target, "v0.4.0"))
	if err == nil {
		for _, e := range entries {
			if e.Name() == "craftpull-windows-amd64.zip" {
				t.Error("broken download must be removed")
			}
		}
	}
}

func TestUpdateDir(t *testing.T) {
	got := UpdateDir(filepath.Join("data"), "v0.4.0")
	want := filepath.Join("data", "update", "v0.4.0")
	if got != want {
		t.Errorf("UpdateDir = %q, want %q", got, want)
	}
}

// Файлы с публичной страницы переносятся в апдейтер вместе с суммами.
func TestConvertAssetsKeepsDigest(t *testing.T) {
	digest := "sha256:abc"
	assets := []model.AssetInfo{
		{
			Name:   "craftpull-windows-amd64.zip",
			URL:    "https://github.com/o/r/releases/download/v0.4.0/craftpull-windows-amd64.zip",
			Size:   123,
			Digest: &digest,
		},
		{Name: "v0.4.0-source.zip", URL: "https://example.invalid/source.zip"},
	}
	out := convertAssets(assets)
	if len(out) != 2 {
		t.Fatalf("ассетов: %d, ожидалось 2", len(out))
	}
	if out[0].Digest != digest {
		t.Errorf("сумма = %q, ожидалась %q", out[0].Digest, digest)
	}
	if out[0].Size != 123 {
		t.Errorf("размер = %d, ожидалось 123", out[0].Size)
	}
	if out[1].Digest != "" {
		t.Errorf("у файла без суммы сумма = %q", out[1].Digest)
	}
}
