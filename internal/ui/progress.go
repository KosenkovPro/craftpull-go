//go:build cgo

// Полоса прогресса с процентом по центру и подробный блок загрузки.
package ui

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/KosenkovPro/craftpull-go/internal/i18n"
	"github.com/KosenkovPro/craftpull-go/internal/model"
)

// progressBar — полоса прогресса, у которой процент нарисован поверх полосы.
type progressBar struct {
	widget.BaseWidget
	value float64
	label string
}

func newProgressBar(value float64, label string) *progressBar {
	p := &progressBar{value: value, label: label}
	p.ExtendBaseWidget(p)
	return p
}

// SetValue обновляет долю выполнения (0..1).
func (p *progressBar) SetValue(v float64) {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	p.value = v
	p.Refresh()
}

// SetLabel меняет подпись по центру полосы.
func (p *progressBar) SetLabel(s string) {
	p.label = s
	p.Refresh()
}

func (p *progressBar) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(colTrack)
	fill := canvas.NewRectangle(colAccent)
	shadow := canvas.NewText(p.label, colTrack)
	shadow.TextSize = 12
	text := canvas.NewText(p.label, color.White)
	text.TextSize = 12
	text.Alignment = fyne.TextAlignCenter
	return &progressRenderer{bar: p, bg: bg, fill: fill, shadow: shadow, text: text}
}

type progressRenderer struct {
	bar    *progressBar
	bg     *canvas.Rectangle
	fill   *canvas.Rectangle
	shadow *canvas.Text
	text   *canvas.Text
}

func (r *progressRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	r.fill.Resize(fyne.NewSize(size.Width*float32(r.bar.value), size.Height))
	r.shadow.Resize(size)
	r.text.Resize(size)
	textH := r.text.MinSize().Height
	r.text.Move(fyne.NewPos(0, (size.Height-textH)/2))
	r.shadow.Move(fyne.NewPos(1, (size.Height-textH)/2+1))
}

func (r *progressRenderer) MinSize() fyne.Size {
	return fyne.NewSize(120, 16)
}

func (r *progressRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.fill, r.shadow, r.text}
}

// Destroy освобождает ресурсы рендерера.
func (r *progressRenderer) Destroy() {}

// tick рисует галочку линиями: в стандартном шрифте нет символа ✓.
func tick(size float32) fyne.CanvasObject {
	first := canvas.NewLine(colOK)
	first.StrokeWidth = 2
	first.Position1 = fyne.NewPos(0, size*0.5)
	first.Position2 = fyne.NewPos(size*0.35, size*0.85)

	second := canvas.NewLine(colOK)
	second.StrokeWidth = 2
	second.Position1 = fyne.NewPos(size*0.35, size*0.85)
	second.Position2 = fyne.NewPos(size, size*0.1)

	box := container.NewWithoutLayout(first, second)
	return box
}

// circle рисует кружок: в стандартном шрифте нет символа ○.
func circle(size float32) fyne.CanvasObject {
	c := canvas.NewCircle(color.Transparent)
	c.StrokeColor = colDim
	c.StrokeWidth = 1.5
	c.Resize(fyne.NewSize(size, size))
	box := container.NewWithoutLayout(c)
	return box
}

func (r *progressRenderer) Refresh() {
	r.fill.FillColor = fillColor(float32(r.bar.value))
	r.text.Text = r.bar.label
	r.shadow.Text = r.bar.label
	canvas.Refresh(r.bar)
}

// progressBlock собирает подробный блок загрузки:
//
//	#1  имя-файла   Качается              2.9 МБ/с
//	[============ 45 % ============]
//	прошло 00:06  осталось ~00:27  19.0 МБ из 99.6 МБ  очередь 1 из 1
func progressBlock(task model.TaskProgress) fyne.CanvasObject {
	number := widget.NewLabel(fmt.Sprintf("#%d", task.Queue))
	number.TextStyle = fyne.TextStyle{Bold: true}

	name := widget.NewLabel(task.Asset)
	name.TextStyle = fyne.TextStyle{Bold: true}

	state := widget.NewLabel(task.State.Label())

	speed := widget.NewLabel(model.HumanSpeed(task.Speed))
	speed.TextStyle = fyne.TextStyle{Bold: true}

	left := container.NewHBox(number, name, state)
	right := container.NewHBox(speed)
	header := container.NewBorder(nil, nil, left, right)

	fraction := task.Fraction()
	bar := newProgressBar(float64(fraction), fmt.Sprintf("%d %%", int(fraction*100)))

	details := container.NewHBox()
	for _, part := range detailParts(task) {
		label := widget.NewLabel(part)
		details.Add(label)
	}

	return container.NewVBox(header, bar, details)
}

// detailParts собирает подробности загрузки: прошло, осталось, объём, очередь.
func detailParts(task model.TaskProgress) []string {
	parts := []string{
		i18n.TF("dl.elapsed", map[string]string{"{t}": model.HumanDuration(task.ElapsedSecs())}),
	}
	if left, ok := task.RemainingSecs(); ok {
		parts = append(parts, i18n.TF("dl.remaining", map[string]string{"{t}": model.HumanDuration(left)}))
	}
	parts = append(parts, i18n.TF("dl.downloaded", map[string]string{
		"{a}": model.HumanSize(task.Downloaded),
		"{b}": model.HumanSize(task.Total),
	}))
	if task.QueueTotal > 0 {
		parts = append(parts, i18n.TF("dl.queue", map[string]string{
			"{n}": fmt.Sprintf("%d", task.Queue),
			"{m}": fmt.Sprintf("%d", task.QueueTotal),
		}))
	}
	return parts
}

// themeColor возвращает цвет темы по имени.
func themeColor(name fyne.ThemeColorName) color.Color { return theme.Color(name) }

// tickOld и circleOld остались в Rust-версии как подсказка по геометрии.
func tickOld(size float32) {}

func circleOld(size float32) {}
