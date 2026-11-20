//go:build cgo

// Картинка, на которую можно нажать: значок репозитория в плитке сетки.
package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

// tappableImage — значок, открывающий карточку репозитория по нажатию.
type tappableImage struct {
	widget.BaseWidget
	image *canvas.Image
	onTap func()
}

// newTappableImage создаёт нажимаемую картинку заданного размера.
func newTappableImage(img *canvas.Image, onTap func()) *tappableImage {
	t := &tappableImage{image: img, onTap: onTap}
	t.ExtendBaseWidget(t)
	return t
}

// Tapped обрабатывает нажатие на значок.
func (t *tappableImage) Tapped(_ *fyne.PointEvent) {
	if t.onTap != nil {
		t.onTap()
	}
}

// CreateRenderer создаёт отрисовку значка.
func (t *tappableImage) CreateRenderer() fyne.WidgetRenderer {
	return &tappableImageRenderer{img: t.image}
}

// tappableImageRenderer — отрисовка нажимаемой картинки.
type tappableImageRenderer struct {
	img *canvas.Image
}

func (r *tappableImageRenderer) Layout(size fyne.Size) {
	r.img.Resize(size)
}

func (r *tappableImageRenderer) MinSize() fyne.Size {
	return r.img.MinSize()
}
func (r *tappableImageRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.img}
}

// Refresh перерисовывает значок.
func (r *tappableImageRenderer) Refresh() {
	canvas.Refresh(r.img)
}

func (r *tappableImageRenderer) Destroy() {}

// tappableBox — нажимаемая область с готовой графикой.
//
// Так сделан уголок у значка репозитория: тонкая линия вместо кнопки,
// чтобы не закрывать сам значок.
type tappableBox struct {
	widget.BaseWidget
	objects []fyne.CanvasObject
	size    fyne.Size
	onTap   func()
}

// newTappableBox создаёт нажимаемую область заданного размера.
func newTappableBox(size fyne.Size, objects []fyne.CanvasObject, onTap func()) *tappableBox {
	t := &tappableBox{objects: objects, size: size, onTap: onTap}
	t.ExtendBaseWidget(t)
	return t
}

// Tapped обрабатывает нажатие.
func (t *tappableBox) Tapped(_ *fyne.PointEvent) {
	if t.onTap != nil {
		t.onTap()
	}
}

// CreateRenderer создаёт отрисовку области.
func (t *tappableBox) CreateRenderer() fyne.WidgetRenderer {
	return &tappableBoxRenderer{objects: t.objects, size: t.size}
}

// tappableBoxRenderer — отрисовка нажимаемой области.
type tappableBoxRenderer struct {
	objects []fyne.CanvasObject
	size    fyne.Size
}

func (r *tappableBoxRenderer) Layout(size fyne.Size) {
	for _, o := range r.objects {
		o.Resize(r.size)
	}
}

func (r *tappableBoxRenderer) MinSize() fyne.Size {
	return r.size
}

func (r *tappableBoxRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

func (r *tappableBoxRenderer) Refresh() {
	for _, o := range r.objects {
		canvas.Refresh(o)
	}
}

func (r *tappableBoxRenderer) Destroy() {}

// chevronGlyph рисует уголок «вниз» двумя линиями.
func chevronGlyph(width float32, tint color.Color) []fyne.CanvasObject {
	first := canvas.NewLine(tint)
	first.StrokeWidth = 1.6
	first.Position1 = fyne.NewPos(0, 0)
	first.Position2 = fyne.NewPos(width/2, width*0.65)

	second := canvas.NewLine(tint)
	second.StrokeWidth = 1.6
	second.Position1 = fyne.NewPos(width/2, width*0.65)
	second.Position2 = fyne.NewPos(width, 0)

	return []fyne.CanvasObject{first, second}
}
