package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ExtractPayload достаёт исполняемый файл из скачанного архива.
//
// Поддерживаются zip (Windows и macOS .app) и tar.gz (Linux). Если файл без
// расширения, это готовый бинарник — он копируется как есть.
func ExtractPayload(archive, destDir string) (string, error) {
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", err
	}
	switch {
	case strings.HasSuffix(archive, ".zip"):
		return extractZip(archive, destDir)
	case strings.HasSuffix(archive, ".tar.gz"), strings.HasSuffix(archive, ".tgz"):
		return extractTarGz(archive, destDir)
	default:
		target := filepath.Join(destDir, filepath.Base(archive))
		if err := copyFile(archive, target); err != nil {
			return "", err
		}
		if err := os.Chmod(target, 0o755); err != nil {
			return "", err
		}
		return target, nil
	}
}

// extractZip достаёт исполняемый файл из zip-архива.
func extractZip(archive, destDir string) (string, error) {
	r, err := zip.OpenReader(archive)
	if err != nil {
		return "", err
	}
	defer r.Close()

	names := make([]string, 0, len(r.File))
	for _, f := range r.File {
		names = append(names, f.Name)
	}
	pick := pickPayload(names)
	if pick == "" {
		return "", fmt.Errorf("в архиве %s нет исполняемого файла", filepath.Base(archive))
	}
	for _, f := range r.File {
		if f.Name != pick {
			continue
		}
		target := filepath.Join(destDir, filepath.Base(pick))
		if err := writeEntry(f, target); err != nil {
			return "", err
		}
		return target, nil
	}
	return "", fmt.Errorf("в архиве %s нет исполняемого файла", filepath.Base(archive))
}

// extractTarGz достаёт исполняемый файл из tar.gz.
func extractTarGz(archive, destDir string) (string, error) {
	f, err := os.Open(archive)
	if err != nil {
		return "", err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	names := make([]string, 0, 16)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if hdr.Typeflag == tar.TypeReg {
			names = append(names, hdr.Name)
		}
	}
	pick := pickPayload(names)
	if pick == "" {
		return "", fmt.Errorf("в архиве %s нет исполняемого файла", filepath.Base(archive))
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	gz2, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gz2.Close()
	tr2 := tar.NewReader(gz2)
	for {
		hdr, err := tr2.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if hdr.Typeflag != tar.TypeReg || hdr.Name != pick {
			continue
		}
		target := filepath.Join(destDir, filepath.Base(pick))
		out, err := os.Create(target)
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(out, tr2); err != nil {
			out.Close()
			return "", err
		}
		out.Close()
		if err := os.Chmod(target, 0o755); err != nil {
			return "", err
		}
		return target, nil
	}
	return "", fmt.Errorf("в архиве %s нет исполняемого файла", filepath.Base(archive))
}

// writeEntry распаковывает один файл из zip-архива.
func writeEntry(f *zip.File, target string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, rc); err != nil {
		return err
	}
	return os.Chmod(target, 0o755)
}

// pickPayload выбирает исполняемый файл среди имён архива.
//
// Порядок предпочтения: .exe для Windows, исполняемый файл внутри .app для
// macOS, файл без расширения с именем craftpull для Linux.
func pickPayload(names []string) string {
	for _, n := range names {
		if strings.HasSuffix(strings.ToLower(n), ".exe") {
			return n
		}
	}
	for _, n := range names {
		if strings.Contains(n, "Contents/MacOS/") {
			return n
		}
	}
	for _, n := range names {
		base := strings.ToLower(filepath.Base(n))
		if base == "craftpull" || strings.HasPrefix(base, "craftpull-") {
			return n
		}
	}
	return ""
}
