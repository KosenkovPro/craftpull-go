//go:build cgo

// Вкладка настроек.
//
// Разделы собраны карточками — так же, как в Windows-версии: заголовок
// раздела, тонкая линия и сами настройки под ней.
package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/KosenkovPro/craftpull-go/internal/config"
	"github.com/KosenkovPro/craftpull-go/internal/i18n"
	"github.com/KosenkovPro/craftpull-go/internal/shortcut"
)

// buildSettings собирает вкладку настроек.
func (u *UI) buildSettings() fyne.CanvasObject {
	cfg := u.mgr.Config()

	folder := widget.NewEntry()
	folder.SetText(cfg.RootDir)
	folderEntry := folder

	speeds := []string{
		i18n.T("settings.no_limit"), "1 MB/s", "2 MB/s", "4 MB/s", "8 MB/s", "16 MB/s",
	}
	speedIndex := 0
	switch {
	case cfg.SpeedLimitKbps == 0:
		speedIndex = 0
	case cfg.SpeedLimitKbps <= 1024:
		speedIndex = 1
	case cfg.SpeedLimitKbps <= 2048:
		speedIndex = 2
	case cfg.SpeedLimitKbps <= 4096:
		speedIndex = 3
	case cfg.SpeedLimitKbps <= 8192:
		speedIndex = 4
	default:
		speedIndex = 5
	}
	speed := widget.NewSelect(speeds, nil)
	speed.SetSelected(speeds[speedIndex])

	parallel := widget.NewSlider(1, float64(config.MaxParallel))
	parallel.SetValue(float64(cfg.MaxParallel))
	parallelValue := labelDim(fmt.Sprintf("%d %s", cfg.MaxParallel,
		i18n.TF("settings.parallel_max", map[string]string{"{n}": fmt.Sprintf("%d", config.MaxParallel)})))
	parallel.OnChanged = func(v float64) {
		parallelValue.SetText(fmt.Sprintf("%d %s", int(v),
			i18n.TF("settings.parallel_max", map[string]string{"{n}": fmt.Sprintf("%d", int(v))})))
	}

	mode := widget.NewSelect([]string{config.DownloadParallel.Label(), config.DownloadSequential.Label()}, nil)
	if cfg.DownloadMode == config.DownloadSequential {
		mode.SetSelected(config.DownloadSequential.Label())
	} else {
		mode.SetSelected(config.DownloadParallel.Label())
	}

	verify := widget.NewCheck(i18n.T("settings.sha"), nil)
	verify.SetChecked(cfg.VerifySHA256)

	token := widget.NewEntry()
	token.SetText(cfg.GitHubToken)
	token.Password = true

	stable := widget.NewCheck(i18n.T("settings.stable"), nil)
	stable.SetChecked(cfg.StableChannel)
	edge := widget.NewCheck(i18n.T("settings.edge"), nil)
	edge.SetChecked(cfg.EdgeChannel)

	deleteOld := widget.NewCheck(i18n.T("settings.delete_old"), nil)
	deleteOld.SetChecked(cfg.DeleteOldVersions)

	closeTray := widget.NewCheck(i18n.T("settings.close_tray"), nil)
	closeTray.SetChecked(cfg.CloseToTray)

	startMin := widget.NewCheck(i18n.T("settings.start_min"), nil)
	startMin.SetChecked(cfg.StartMinimized)

	lang := widget.NewSelect([]string{i18n.LangName(i18n.EN), i18n.LangName(i18n.RU)}, nil)
	if cfg.Lang == i18n.RU {
		lang.SetSelected(i18n.LangName(i18n.RU))
	} else {
		lang.SetSelected(i18n.LangName(i18n.EN))
	}

	// Режим отображения — кнопками, как в Windows-версии.
	viewButtons := container.NewHBox()
	viewModes := []config.ViewMode{config.ViewGrid, config.ViewList, config.ViewLight}
	for _, mode := range viewModes {
		mode := mode
		btn := widget.NewButton(mode.Label(), func() {
			cfg := u.mgr.Config()
			cfg.ViewMode = mode
			_ = u.mgr.SaveConfig(cfg)
			u.st.SetView(mode)
			u.Rebuild()
		})
		if mode == cfg.ViewMode {
			btn.Importance = widget.HighImportance
		}
		viewButtons.Add(btn)
	}

	save := widget.NewButton(i18n.T("common.save"), func() {
		newCfg := u.mgr.Config()
		newCfg.RootDir = folderEntry.Text
		switch speed.Selected {
		case "1 MB/s":
			newCfg.SpeedLimitKbps = 1024
		case "2 MB/s":
			newCfg.SpeedLimitKbps = 2048
		case "4 MB/s":
			newCfg.SpeedLimitKbps = 4096
		case "8 MB/s":
			newCfg.SpeedLimitKbps = 8192
		case "16 MB/s":
			newCfg.SpeedLimitKbps = 16384
		default:
			newCfg.SpeedLimitKbps = 0
		}
		newCfg.MaxParallel = int(parallel.Value)
		if mode.Selected == config.DownloadSequential.Label() {
			newCfg.DownloadMode = config.DownloadSequential
		} else {
			newCfg.DownloadMode = config.DownloadParallel
		}
		newCfg.VerifySHA256 = verify.Checked
		newCfg.GitHubToken = token.Text
		newCfg.StableChannel = stable.Checked
		newCfg.EdgeChannel = edge.Checked
		newCfg.DeleteOldVersions = deleteOld.Checked
		newCfg.CloseToTray = closeTray.Checked
		newCfg.StartMinimized = startMin.Checked
		if lang.Selected == i18n.LangName(i18n.RU) {
			newCfg.Lang = i18n.RU
		} else {
			newCfg.Lang = i18n.EN
		}
		if err := u.mgr.SaveConfig(newCfg); err != nil {
			u.st.SetNotice(err.Error())
			return
		}
		u.st.SetView(newCfg.ViewMode)
		u.st.SetLang(newCfg.Lang)
		u.st.SetNotice(i18n.T("notify.saved"))
		u.Rebuild()
	})

	shortcutBtn := widget.NewButton(i18n.T("settings.shortcut"), func() {
		path, err := shortcut.Install()
		if err != nil {
			u.st.SetNotice(err.Error())
			return
		}
		u.st.SetNotice(i18n.TF("notify.shortcut_ok", map[string]string{"{p}": path}))
		u.Rebuild()
	})
	openData := widget.NewButton(i18n.T("settings.open_data"), func() {
		_ = shortcut.OpenDataDir()
	})
	openLog := widget.NewButton(i18n.T("settings.open_log"), func() {
		_ = shortcut.OpenLogFile()
	})

	// Обновление самой программы: одна кнопка на всю операцию.
	// Кнопка установки активна, когда обновление уже найдено.
	upd := u.SelfUpdate()
	busy := u.SelfBusy()
	updateText := i18n.T("settings.update_none")
	if upd != nil {
		updateText = i18n.TF("settings.update_to", map[string]string{"{v}": upd.Version})
	}
	updateBtn := widget.NewButton(updateText, func() {
		u.installSelfUpdate()
	})
	if upd != nil && !busy {
		updateBtn.Importance = widget.HighImportance
	} else {
		updateBtn.Disable()
	}
	checkBtn := widget.NewButton(i18n.T("settings.update_check"), func() {
		u.checkSelfUpdate(false)
	})
	if busy {
		checkBtn.Disable()
	}
	updateSection := container.NewVBox(
		rowOf(i18n.T("settings.update_current"), labelDim(fmt.Sprintf("%s", Version))),
		rowOf(i18n.T("settings.update_repo"), labelDim(updateRepo(cfg))),
		rowOf("", container.NewHBox(checkBtn, updateBtn)),
		labelDim(i18n.T("settings.update_hint")),
	)

	body := container.NewVBox(
		sectionCard(i18n.T("settings.language"),
			container.NewVBox(
				rowOf(i18n.T("settings.language"), container.New(widthLayout{width: 250}, lang)),
				rowOf(i18n.T("repo.view_mode"), viewButtons),
			)),
		sectionCard(i18n.T("settings.folder"),
			rowOf(i18n.T("settings.folder"), folder)),
		sectionCard(i18n.T("settings.network"),
			container.NewVBox(
				rowOf(i18n.T("settings.limit"), container.New(widthLayout{width: 250}, speed)),
				rowOf(i18n.T("settings.parallel"), container.NewHBox(parallel, parallelValue)),
				rowOf(i18n.T("download.mode"), container.New(widthLayout{width: 250}, mode)),
				rowOf("", verify),
				rowOf(i18n.T("settings.token"), token),
			)),
		sectionCard(i18n.T("settings.channels"),
			container.NewVBox(stable, edge)),
		sectionCard(i18n.T("settings.behaviour"),
			container.NewVBox(deleteOld, closeTray, startMin)),
		sectionCard(i18n.T("settings.update"),
			updateSection),
		sectionCard(i18n.T("settings.install"),
			container.NewHBox(shortcutBtn, openData, openLog)),
	)

	// Заголовок страницы и липкая кнопка сохранения.
	head := heading(i18n.T("settings.title"))
	// Кнопка сохранения липнет к низу: её видно всегда, даже если настроек
	// больше, чем помещается в окно.
	return container.NewBorder(container.NewVBox(head), container.NewHBox(save), nil, nil,
		container.NewScroll(body))
}

// sectionCard собирает карточку раздела настроек: заголовок, линия, содержимое.
func sectionCard(title string, content fyne.CanvasObject) fyne.CanvasObject {
	divider := canvas.NewRectangle(colBorder)
	divider.SetMinSize(fyne.NewSize(0, 1))
	rows := container.NewVBox(labelText(title, true), divider, content)
	rect := canvas.NewRectangle(colCard)
	rect.StrokeColor = colBorder
	rect.StrokeWidth = 1
	rect.CornerRadius = 12
	return container.NewStack(rect, container.NewPadded(rows))
}

// rowOf собирает строку «подпись — значение».
func rowOf(label string, value fyne.CanvasObject) fyne.CanvasObject {
	text := labelDim(label)
	return container.NewBorder(nil, nil, text, nil, value)
}
