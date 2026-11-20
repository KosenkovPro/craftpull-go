//go:build cgo

// Картинки интерфейса: значок программы и значки репозиториев.
package ui

import (
	"bytes"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"

	"github.com/KosenkovPro/craftpull-go/internal/icon"
)

// programIcon возвращает значок программы заданного размера.
//
// Размер задан явно: Fyne рисует картинку по её собственному размеру, и без
// этого значок просто не появляется на экране.
func programIcon(size float32) *canvas.Image {
	img := canvas.NewImageFromImage(icon.Image(96))
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(size, size))
	img.Resize(fyne.NewSize(size, size))
	return img
}

// repoIconImage возвращает значок репозитория: собственный, если он
// скачан, иначе значок программы.
func repoIconImage(path *string, size float32) *canvas.Image {
	if path != nil && *path != "" {
		if data, err := os.ReadFile(*path); err == nil {
			if decoded, _, err := image.Decode(bytes.NewReader(data)); err == nil {
				img := canvas.NewImageFromImage(decoded)
				img.FillMode = canvas.ImageFillContain
				img.SetMinSize(fyne.NewSize(size, size))
				img.Resize(fyne.NewSize(size, size))
				return img
			}
		}
	}
	return programIcon(size)
}
