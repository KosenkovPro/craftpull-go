//go:build cgo

package ui

import (
	"sync"

	"fyne.io/fyne/v2"
)

// defaultWindowSize — размер окна по умолчанию.
func defaultWindowSize() fyne.Size {
	return fyne.NewSize(1296, 879)
}

// windowHost хранит текущее окно программы.
//
// Кнопка закрытия в Fyne уничтожает окно, поэтому сворачивание в трей
// устроено так: вместо закрытого окна создаётся новое скрытое окно с тем же
// содержимым — приложение остаётся жить в трее и его можно снова показать.
type windowHost struct {
	mu  sync.Mutex
	win fyne.Window
}

// current возвращает текущее окно.
func (h *windowHost) current() fyne.Window {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.win
}

// set запоминает новое окно.
func (h *windowHost) set(w fyne.Window) {
	h.mu.Lock()
	h.win = w
	h.mu.Unlock()
}

// replace создаёт скрытое окно вместо закрытого и запоминает его.
func (h *windowHost) replace(a fyne.App, ui *UI, title string, iconRes fyne.Resource) {
	w := a.NewWindow(title)
	w.Resize(defaultWindowSize())
	w.SetIcon(iconRes)
	w.SetContent(ui.Build(w))
	w.Hide()
	h.set(w)
	w.SetOnClosed(func() {
		if !ui.mgr.Config().CloseToTray {
			return
		}
		h.replace(a, ui, title, iconRes)
	})
}
