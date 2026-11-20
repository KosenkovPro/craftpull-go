//go:build cgo

// Подсказка при наведении: короткое объяснение, которое появляется,
// когда указатель стоит на элементе.
package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// tipLabel — подпись с подсказкой.
//
// Сама подпись живёт в сайдбаре и не пересоздаётся при перерисовке,
// поэтому подсказка появляется сразу, без лишнего движения мыши.
type tipLabel struct {
	widget.BaseWidget
	label *widget.Label
	tip   string
	popup *widget.PopUp
}

// newTipLabel создаёт подпись с подсказкой.
func newTipLabel(text, tip string) *tipLabel {
	l := widget.NewLabel(text)
	l.Importance = widget.LowImportance
	t := &tipLabel{label: l, tip: tip}
	t.ExtendBaseWidget(t)
	return t
}

// CreateRenderer создаёт отрисовку подписи.
func (t *tipLabel) CreateRenderer() fyne.WidgetRenderer {
	return &tipLabelRenderer{label: t.label}
}

// tipLabelRenderer — отрисовка подписи с подсказкой.
type tipLabelRenderer struct {
	label *widget.Label
}

func (r *tipLabelRenderer) Layout(size fyne.Size) {
	r.label.Resize(size)
}

func (r *tipLabelRenderer) MinSize() fyne.Size {
	return r.label.MinSize()
}

func (r *tipLabelRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.label}
}

func (r *tipLabelRenderer) Refresh() {
	r.label.Refresh()
}

func (r *tipLabelRenderer) Destroy() {}

// MouseIn показывает подсказку.
func (t *tipLabel) MouseIn(_ *desktop.MouseEvent) { t.show() }

// MouseMoved ничего не делает: подсказка уже показана.
func (t *tipLabel) MouseMoved(_ *desktop.MouseEvent) {}

// MouseOut прячет подсказку.
func (t *tipLabel) MouseOut() { t.hide() }

// show показывает всплывающую подсказку под подписью.
func (t *tipLabel) show() {
	if t.tip == "" || t.popup != nil {
		return
	}
	app := fyne.CurrentApp()
	if app == nil || app.Driver() == nil {
		return
	}
	target := fyne.CanvasObject(t.label)
	screen := app.Driver().CanvasForObject(target)
	if screen == nil {
		return
	}
	t.popup = widget.NewPopUp(tipContent(t.tip), screen)
	t.popup.ShowAtRelativePosition(fyne.NewPos(0, t.label.MinSize().Height+6), target)
}

// hide прячет всплывающую подсказку.
func (t *tipLabel) hide() {
	if t.popup == nil {
		return
	}
	t.popup.Hide()
	t.popup = nil
}

// shown сообщает, показывается ли подсказка сейчас.
func (t *tipLabel) shown() bool { return t.popup != nil }

// tipContent собирает оформление подсказки: карточка с пояснением.
func tipContent(text string) fyne.CanvasObject {
	rect := canvas.NewRectangle(colPanel)
	rect.StrokeColor = colBorder
	rect.StrokeWidth = 1
	rect.CornerRadius = 8

	label := widget.NewLabel(text)
	label.Importance = widget.LowImportance
	label.Wrapping = fyne.TextWrapWord

	body := container.New(widthLayout{width: 320}, label)
	return container.NewStack(rect, container.NewPadded(body))
}
