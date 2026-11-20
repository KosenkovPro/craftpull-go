//go:build cgo

package ui

import (
	"context"
	"time"

	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/KosenkovPro/craftpull-go/internal/app"
	"github.com/KosenkovPro/craftpull-go/internal/buildinfo"
	"github.com/KosenkovPro/craftpull-go/internal/i18n"
	"github.com/KosenkovPro/craftpull-go/internal/icon"
	"github.com/KosenkovPro/craftpull-go/internal/manager"
)

// Run запускает графическое окно программы.
func Run(mgr *manager.Manager, st *app.State, opts Options) error {
	a := fyneapp.New()

	iconRes := fyne.NewStaticResource("craftpull", icon.PNG(256))
	a.SetIcon(iconRes)

	title := buildinfo.Name + " — " + buildinfo.Subtitle
	win := a.NewWindow(title)
	win.Resize(defaultWindowSize())
	win.SetIcon(iconRes)

	ui := NewUI(mgr, st)
	win.SetContent(ui.Build(win))

	host := &windowHost{win: win}

	// Значок в трее: окно, пауза, остановка, выход.
	ui.SetTrayRefresh(setupTray(a, host, mgr, iconRes))

	// Закрытие окна сворачивает программу в трей, если это разрешено.
	win.SetOnClosed(func() {
		if !mgr.Config().CloseToTray {
			return
		}
		host.replace(a, ui, title, iconRes)
	})

	if opts.Popup != "" {
		st.OpenReleases(opts.Popup)
	}
	if opts.Hero != "" {
		st.SetHero(opts.Hero)
	}
	ui.Rebuild()

	if opts.AutoRefresh {
		go func() {
			mgr.RefreshAll(context.Background(), opts.ForceRefresh)
		}()
	}
	if opts.Download != "" {
		go func() {
			mgr.DownloadRepo(context.Background(), opts.Download, manager.DownloadRequest{Targets: true})
		}()
	}

	// Обновление интерфейса: прогресс, очередь, журнал.
	ticker := time.NewTicker(700 * time.Millisecond)
	go func() {
		for range ticker.C {
			fyne.Do(func() { ui.Tick() })
		}
	}()

	// Запуск в трее: окно остаётся скрытым, программа живёт в фоне.
	if opts.Hidden {
		a.Run()
		return nil
	}

	win.ShowAndRun()
	return nil
}

// showDialog показывает простое модальное окно с одним полем и кнопкой.
func showDialog(parent fyne.Window, title string, content fyne.CanvasObject, onConfirm func()) *dialog.ConfirmDialog {
	dlg := dialog.NewCustomConfirm(title, i18n.T("common.save"), i18n.T("common.cancel"), content,
		func(ok bool) {
			if ok {
				onConfirm()
			}
		}, parent)
	dlg.Show()
	return dlg
}

// dialogField — поле ввода внутри диалога.
func dialogField(placeholder string) *widget.Entry {
	e := widget.NewEntry()
	e.SetPlaceHolder(placeholder)
	return e
}
