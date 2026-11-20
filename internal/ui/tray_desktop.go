//go:build cgo && (windows || darwin)

package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"

	"github.com/KosenkovPro/craftpull-go/internal/i18n"
	"github.com/KosenkovPro/craftpull-go/internal/manager"
)

// setupTray создаёт значок в системном лотке: показать окно, пауза,
// остановка загрузок, выход.
//
// Возвращает функцию пересборки меню — она нужна, чтобы подписи
// соответствовали текущему языку интерфейса.
func setupTray(a fyne.App, host *windowHost, mgr *manager.Manager, iconRes fyne.Resource) func() {
	desk, ok := a.(desktop.App)
	if !ok {
		return nil
	}

	var refresh func()

	build := func() *fyne.Menu {
		pauseLabel := i18n.T("tray.pause")
		if mgr.Control().IsPaused() {
			pauseLabel = i18n.T("tray.resume")
		}

		show := fyne.NewMenuItem(i18n.T("tray.show"), func() {
			fyne.Do(func() {
				if w := host.current(); w != nil {
					w.Show()
					w.RequestFocus()
				}
			})
		})
		pause := fyne.NewMenuItem(pauseLabel, func() {
			mgr.TogglePause()
			refresh()
		})
		stop := fyne.NewMenuItem(i18n.T("tray.stop"), func() {
			mgr.StopAll()
			refresh()
		})
		exit := fyne.NewMenuItem(i18n.T("tray.exit"), func() {
			fyne.Do(func() { a.Quit() })
		})

		return fyne.NewMenu("", show, pause, stop, fyne.NewMenuItemSeparator(), exit)
	}

	refresh = func() {
		desk.SetSystemTrayWindow(host.current())
		desk.SetSystemTrayMenu(build())
		desk.SetSystemTrayIcon(iconRes)
	}
	refresh()
	return refresh
}
