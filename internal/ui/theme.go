//go:build cgo

// Цвета и общие элементы оформления интерфейса.
package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
)

// Палитра интерфейса.
var (
	colAccent   = color.NRGBA{R: 0x2F, G: 0x8E, B: 0xF7, A: 0xFF}
	colOK       = color.NRGBA{R: 0x35, G: 0xC7, B: 0x8A, A: 0xFF}
	colWarn     = color.NRGBA{R: 0xF2, G: 0xB0, B: 0x3C, A: 0xFF}
	colEdge     = color.NRGBA{R: 0xC7, G: 0x7B, B: 0xF2, A: 0xFF}
	colError    = color.NRGBA{R: 0xE8, G: 0x54, B: 0x54, A: 0xFF}
	colDim      = color.NRGBA{R: 0x8A, G: 0x93, B: 0xA5, A: 0xFF}
	colCard     = color.NRGBA{R: 0x24, G: 0x2A, B: 0x35, A: 0xFF}
	colCardSoft = color.NRGBA{R: 0x2C, G: 0x33, B: 0x40, A: 0xFF}
	colBorder   = color.NRGBA{R: 0x3A, G: 0x43, B: 0x54, A: 0xFF}
	colTrack    = color.NRGBA{R: 0x18, G: 0x1C, B: 0x24, A: 0xFF}
)

// fillColor возвращает цвет заливки по доле выполнения.
func fillColor(fraction float32) color.Color {
	switch {
	case fraction >= 1:
		return colOK
	case fraction >= 0.5:
		return colAccent
	default:
		return color.NRGBA{R: 0x4A, G: 0x9B, B: 0xF5, A: 0xFF}
	}
}

// stateColor возвращает цвет состояния задачи.
func stateColor(state string) color.Color {
	switch state {
	case "error":
		return colError
	case "ok":
		return colOK
	case "warn":
		return colWarn
	case "edge":
		return colEdge
	}
	return colDim
}

// sizeOf возвращает ширину элемента по родителю.
func sizeOf(parent fyne.CanvasObject) fyne.Size {
	if parent == nil {
		return fyne.NewSize(100, 20)
	}
	return parent.Size()
}
