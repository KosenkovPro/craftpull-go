//go:build cgo

// Вкладка журнала: список проверок, легенда загрузок и записи журнала.
package ui

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/KosenkovPro/craftpull-go/internal/i18n"
	"github.com/KosenkovPro/craftpull-go/internal/manager"
	"github.com/KosenkovPro/craftpull-go/internal/model"
)

// buildLogs собирает вкладку журнала.
func (u *UI) buildLogs() fyne.CanvasObject {
	open := widget.NewButton(i18n.T("log.open"), func() {
		u.st.SetNotice(i18n.T("log.open"))
	})
	clear := widget.NewButton(i18n.T("log.clear"), func() {
		u.mgr.ClearLogs()
		u.Rebuild()
	})
	head := container.NewBorder(nil, nil,
		heading(i18n.T("nav.logs")),
		container.NewHBox(open, clear),
	)

	body := container.NewVBox(
		u.buildChecks(),
		u.buildDownloads(),
		u.buildEntries(),
	)
	return container.NewBorder(head, nil, nil, nil, container.NewScroll(body))
}

// buildChecks собирает блок «Проверка репозиториев».
func (u *UI) buildChecks() fyne.CanvasObject {
	checks := u.mgr.Checks()
	title := labelText(i18n.T("check.title"), true)

	if checks.Total() == 0 {
		return container.NewVBox(title, labelDim(i18n.T("check.waiting")))
	}

	fraction := checks.Fraction()
	bar := newProgressBar(float64(fraction), fmt.Sprintf("%d %%", int(fraction*100)))
	counter := labelDim(i18n.TF("check.progress", map[string]string{
		"{n}": fmt.Sprintf("%d", checks.Done()),
		"{m}": fmt.Sprintf("%d", checks.Total()),
	}))

	list := container.NewVBox()
	for _, item := range checks.Items {
		list.Add(u.checkRow(item))
	}

	return container.NewVBox(title, bar, counter, list)
}

// checkRow собирает строку одного репозитория в списке проверок.
func (u *UI) checkRow(item manager.CheckItem) fyne.CanvasObject {
	mark := canvasTick(item.Status)
	detail := item.Detail
	if detail == "" {
		switch item.Status {
		case manager.CheckRunning:
			detail = i18n.T("check.in_progress")
		case manager.CheckWaiting:
			detail = i18n.T("check.waiting")
		}
	}
	row := container.NewBorder(nil, nil,
		container.NewHBox(mark, widget.NewLabel(item.Slug)),
		labelDim(detail),
	)
	return row
}

// canvasTick рисует значок статуса проверки: галочку или кружок.
func canvasTick(status manager.CheckStatus) fyne.CanvasObject {
	switch status {
	case manager.CheckDone:
		return tick(16)
	case manager.CheckFailed:
		return tick(16)
	default:
		return circle(16)
	}
}

// buildDownloads собирает легенду загрузок.
func (u *UI) buildDownloads() fyne.CanvasObject {
	title := labelText(i18n.T("downloads.title"), true)
	tasks := u.mgr.Tasks().All()
	if len(tasks) == 0 {
		return container.NewVBox(title, labelDim(i18n.T("downloads.empty")))
	}
	list := container.NewVBox()
	for _, t := range tasks {
		list.Add(progressBlock(t))
	}
	return container.NewVBox(title, list)
}

// buildEntries собирает записи журнала.
func (u *UI) buildEntries() fyne.CanvasObject {
	title := labelText(i18n.T("log.title"), true)
	entries := u.mgr.Logs()
	if len(entries) == 0 {
		return container.NewVBox(title, labelDim(i18n.T("log.empty")))
	}
	list := container.NewVBox()
	for i := len(entries) - 1; i >= 0 && i > len(entries)-200; i-- {
		entry := entries[i]
		text := canvas.NewText(fmt.Sprintf("%s  %s", entry.Time, entry.Text), logLevelColor(entry.Level))
		text.TextSize = 12
		list.Add(text)
	}
	return container.NewVBox(title, list)
}

// logLevelColor возвращает цвет уровня записи журнала.
func logLevelColor(level model.LogLevel) color.Color {
	switch level {
	case model.LogOk:
		return colOK
	case model.LogWarn:
		return colWarn
	case model.LogError:
		return colError
	}
	return colDim
}
