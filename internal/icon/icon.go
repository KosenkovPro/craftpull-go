// Package icon — процедурная иконка программы: рисуется кодом, без файлов.
//
// Тот же значок, что и в Rust-версии: скруглённый синий квадрат со стрелкой
// вниз. Формат PNG собирается прямо в памяти.
package icon

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
)

// PNG возвращает иконкуprogramмы в формате PNG.
func PNG(size int) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	drawIcon(img, size)

	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// drawIcon рисует иконку: скруглённый квадрат и стрелку вниз.
func drawIcon(img *image.NRGBA, size int) {
	s := float64(size)
	radius := s * 0.19
	accent := color.NRGBA{R: 0x2F, G: 0x8E, B: 0xF7, A: 0xFF}
	light := color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			fx, fy := float64(x)+0.5, float64(y)+0.5
			if insideRounded(fx, fy, s, s, radius) {
				img.SetNRGBA(x, y, accent)
			}
		}
	}

	// Стрелка вниз: стержень и наконечник.
	stemHalf := s * 0.055
	stemTop := s * 0.24
	stemBottom := s * 0.56
	headHalf := s * 0.22
	headTop := s * 0.46
	headBottom := s * 0.76
	cx := s / 2

	for y := 0; y < size; y++ {
		fy := float64(y) + 0.5
		for x := 0; x < size; x++ {
			fx := float64(x) + 0.5
			if fy < stemTop || fy > headBottom {
				continue
			}
			inside := false
			if fy <= stemBottom && math.Abs(fx-cx) <= stemHalf {
				inside = true
			}
			if fy >= headTop {
				// Наконечник: треугольник, сужающийся к низу.
				t := (fy - headTop) / (headBottom - headTop)
				half := headHalf * (1 - t)
				if math.Abs(fx-cx) <= half {
					inside = true
				}
			}
			if inside {
				img.SetNRGBA(x, y, light)
			}
		}
	}
}

// insideRounded проверяет, что точка внутри скруглённого прямоугольника.
func insideRounded(x, y, w, h, r float64) bool {
	if x < 0 || y < 0 || x > w || y > h {
		return false
	}
	cx := math.Min(math.Max(x, r), w-r)
	cy := math.Min(math.Max(y, r), h-r)
	dx, dy := x-cx, y-cy
	return dx*dx+dy*dy <= r*r
}
