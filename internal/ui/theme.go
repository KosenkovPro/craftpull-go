//go:build cgo

// Оформление интерфейса: палитра, тема Fyne и общие мелкие элементы.
//
// Палитра совпадает с Windows-версией (Rust): тот же тёмный фон, те же
// акценты, те же карточки с рамками. Тема подключается к приложению целиком,
// поэтому обычные элементы Fyne — кнопки, поля ввода, списки — сразу
// выглядят как в Windows-версии.
package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Палитра интерфейса.
var (
	colBG       = color.NRGBA{R: 0x0E, G: 0x11, B: 0x16, A: 0xFF}
	colPanel    = color.NRGBA{R: 0x14, G: 0x19, B: 0x22, A: 0xFF}
	colCard     = color.NRGBA{R: 0x18, G: 0x1E, B: 0x28, A: 0xFF}
	colCardSoft = color.NRGBA{R: 0x11, G: 0x15, B: 0x1C, A: 0xFF}
	colBorder   = color.NRGBA{R: 0x26, G: 0x2F, B: 0x3D, A: 0xFF}
	colText     = color.NRGBA{R: 0xE6, G: 0xED, B: 0xF3, A: 0xFF}
	colDim      = color.NRGBA{R: 0x8B, G: 0x98, B: 0xA9, A: 0xFF}
	colAccent   = color.NRGBA{R: 0x4C, G: 0x8D, B: 0xFF, A: 0xFF}
	colOK       = color.NRGBA{R: 0x34, G: 0xD3, B: 0x99, A: 0xFF}
	colWarn     = color.NRGBA{R: 0xFB, G: 0xBF, B: 0x24, A: 0xFF}
	colError    = color.NRGBA{R: 0xF8, G: 0x71, B: 0x71, A: 0xFF}
	colEdge     = color.NRGBA{R: 0xC0, G: 0x84, B: 0xFC, A: 0xFF}
	colTrack    = color.NRGBA{R: 0x0A, G: 0x0D, B: 0x11, A: 0xFF}
	colHover    = color.NRGBA{R: 0x1C, G: 0x24, B: 0x30, A: 0xFF}
	colPressed  = color.NRGBA{R: 0x22, G: 0x30, B: 0x46, A: 0xFF}
)

// craftTheme — тема приложения: тёмная палитра Windows-версии.
type craftTheme struct{}

// Color возвращает цвет элемента по имени.
func (craftTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return colBG
	case theme.ColorNameButton:
		return colPanel
	case theme.ColorNameDisabledButton:
		return colCardSoft
	case theme.ColorNameDisabled:
		return colDim
	case theme.ColorNameError:
		return colError
	case theme.ColorNameFocus:
		return colAccent
	case theme.ColorNameForeground:
		return colText
	case theme.ColorNameForegroundOnPrimary:
		return color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	case theme.ColorNameForegroundOnSuccess:
		return colBG
	case theme.ColorNameForegroundOnWarning:
		return colBG
	case theme.ColorNameHeaderBackground:
		return colPanel
	case theme.ColorNameHover:
		return colHover
	case theme.ColorNameHyperlink:
		return colAccent
	case theme.ColorNameInputBackground:
		return colTrack
	case theme.ColorNameInputBorder:
		return colBorder
	case theme.ColorNameMenuBackground:
		return colPanel
	case theme.ColorNameOverlayBackground:
		return colCardSoft
	case theme.ColorNamePlaceHolder:
		return color.NRGBA{R: 0x5A, G: 0x66, B: 0x75, A: 0xFF}
	case theme.ColorNamePressed:
		return colPressed
	case theme.ColorNamePrimary:
		return colAccent
	case theme.ColorNameScrollBar:
		return color.NRGBA{R: 0x2E, G: 0x39, B: 0x49, A: 0xFF}
	case theme.ColorNameScrollBarBackground:
		return colBG
	case theme.ColorNameSelection:
		return colAccent
	case theme.ColorNameSeparator:
		return colBorder
	case theme.ColorNameShadow:
		return color.Transparent
	case theme.ColorNameSuccess:
		return colOK
	case theme.ColorNameWarning:
		return colWarn
	}
	return theme.DarkTheme().Color(name, variant)
}

// Icon возвращает значок из стандартного набора Fyne.
func (craftTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DarkTheme().Icon(name)
}

// Font возвращает шрифт из стандартного набора Fyne.
func (craftTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DarkTheme().Font(style)
}

// Size возвращает размер элемента по имени.
func (craftTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameText:
		return 13
	case theme.SizeNameHeadingText:
		return 22
	case theme.SizeNameSubHeadingText:
		return 16
	case theme.SizeNameCaptionText:
		return 11
	case theme.SizeNamePadding:
		return 4
	case theme.SizeNameInnerPadding:
		return 8
	case theme.SizeNameInlineIcon:
		return 18
	case theme.SizeNameButtonRadius:
		return 8
	case theme.SizeNameInputRadius:
		return 6
	case theme.SizeNameCardRadius:
		return 12
	case theme.SizeNameSeparatorThickness:
		return 1
	}
	return theme.DarkTheme().Size(name)
}

// applyTheme подключает палитру приложения к текущему окну.
func applyTheme() {
	if app := fyne.CurrentApp(); app != nil && app.Settings() != nil {
		app.Settings().SetTheme(craftTheme{})
	}
}

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

// labelText создаёт обычную подпись.
func labelText(text string, bold bool) *widget.Label {
	l := widget.NewLabel(text)
	l.TextStyle = fyne.TextStyle{Bold: bold}
	return l
}

// labelDim создаёт приглушённую подпись.
func labelDim(text string) *widget.Label {
	l := widget.NewLabel(text)
	l.Importance = widget.LowImportance
	return l
}

// labelAccent создаёт подпись акцентным цветом.
func labelAccent(text string, bold bool) *widget.Label {
	l := widget.NewLabel(text)
	l.TextStyle = fyne.TextStyle{Bold: bold}
	l.Importance = widget.HighImportance
	return l
}

// labelGreen создаёт подпись зелёным цветом.
func labelGreen(text string, bold bool) *widget.Label {
	l := widget.NewLabel(text)
	l.TextStyle = fyne.TextStyle{Bold: bold}
	l.Importance = widget.SuccessImportance
	return l
}

// labelWarn создаёт подпись жёлтым цветом.
func labelWarn(text string, bold bool) *widget.Label {
	l := widget.NewLabel(text)
	l.TextStyle = fyne.TextStyle{Bold: bold}
	l.Importance = widget.WarningImportance
	return l
}

// labelError создаёт подпись красным цветом.
func labelError(text string, bold bool) *widget.Label {
	l := widget.NewLabel(text)
	l.TextStyle = fyne.TextStyle{Bold: bold}
	l.Importance = widget.DangerImportance
	return l
}

// heading создаёт крупный заголовок раздела.
func heading(text string) *widget.Label {
	l := widget.NewLabel(text)
	l.TextStyle = fyne.TextStyle{Bold: true}
	l.SizeName = theme.SizeNameHeadingText
	return l
}

// textColored — короткий текст заданным цветом: подходит для меток в одну
// строку (версия, канал, имя автора).
func textColored(text string, tint color.Color, size float32, bold bool) fyne.CanvasObject {
	t := canvas.NewText(text, tint)
	t.TextSize = size
	t.TextStyle = fyne.TextStyle{Bold: bold}
	return t
}

// pill — небольшая метка с рамкой: так выглядит статус в шапке и метки
// версий в карточках.
func pill(text string, tint color.Color) fyne.CanvasObject {
	rect := canvas.NewRectangle(colCard)
	rect.StrokeColor = tint
	rect.StrokeWidth = 1
	rect.CornerRadius = 8
	return container.NewStack(rect, container.NewPadded(center(textColored(text, tint, 11, false))))
}

// card — карточка с фоном и рамкой, как в Windows-версии.
func card(content fyne.CanvasObject) fyne.CanvasObject {
	rect := canvas.NewRectangle(colCard)
	rect.StrokeColor = colBorder
	rect.StrokeWidth = 1
	rect.CornerRadius = 12
	return container.NewStack(rect, container.NewPadded(content))
}

// cardWithBorder — карточка с рамкой заданного цвета: так выделена
// раскрытая карточка репозитория.
func cardWithBorder(content fyne.CanvasObject, border color.Color) fyne.CanvasObject {
	rect := canvas.NewRectangle(colCard)
	rect.StrokeColor = border
	rect.StrokeWidth = 1
	rect.CornerRadius = 12
	return container.NewStack(rect, container.NewPadded(content))
}

// chip рисует маленькую метку-таблетку.
func chip(text string, tint color.Color) *canvas.Text {
	t := canvas.NewText(text, tint)
	t.TextSize = 11
	return t
}

// centerLayout — раскладывает объекты по центру контейнера.
type centerLayout struct{}

func (centerLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objs {
		s := o.MinSize()
		o.Resize(s)
		o.Move(fyne.NewPos((size.Width-s.Width)/2, (size.Height-s.Height)/2))
	}
}

func (centerLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	if len(objs) == 0 {
		return fyne.NewSize(0, 0)
	}
	return objs[0].MinSize()
}

// center — объект по центру контейнера.
func center(obj fyne.CanvasObject) fyne.CanvasObject {
	return container.New(centerLayout{}, obj)
}

// cornerLayout — объект на весь контейнер, а последний — в углу.
//
// Используется для плитки сетки: галочка стоит в правом верхнем углу.
type cornerLayout struct {
	offsetX float32
	offsetY float32
}

func (l cornerLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	for i, o := range objs {
		if i == len(objs)-1 && len(objs) > 1 {
			s := o.MinSize()
			o.Resize(s)
			o.Move(fyne.NewPos(size.Width-s.Width-l.offsetX, l.offsetY))
			continue
		}
		o.Resize(size)
		o.Move(fyne.NewPos(0, 0))
	}
}

func (l cornerLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	if len(objs) == 0 {
		return fyne.NewSize(0, 0)
	}
	return objs[0].MinSize()
}
