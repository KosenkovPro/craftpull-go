//go:build cgo

// Вкладка настроек.
package ui

import (
	"context"
	"fmt"
	"net/http"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/KosenkovPro/craftpull-go/internal/config"
	"github.com/KosenkovPro/craftpull-go/internal/i18n"
	"github.com/KosenkovPro/craftpull-go/internal/selfupdate"
	"github.com/KosenkovPro/craftpull-go/internal/shortcut"
)

// buildSettings собирает вкладку настроек.
func (u *UI) buildSettings() fyne.CanvasObject {
	cfg := u.mgr.Config()

	folder := widget.NewEntry()
	folder.SetText(cfg.RootDir)
	folderEntry := folder

	speed := widget.NewSelect([]string{
		i18n.T("settings.no_limit"), "1 MB/s", "2 MB/s", "4 MB/s", "8 MB/s", "16 MB/s",
	}, nil)
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
	speed.SetSelected([]string{
		i18n.T("settings.no_limit"), "1 MB/s", "2 MB/s", "4 MB/s", "8 MB/s", "16 MB/s",
	}[speedIndex])

	parallel := widget.NewSlider(1, float64(config.MaxParallel))
	parallel.SetValue(float64(cfg.MaxParallel))
	parallelValue := widget.NewLabel(fmt.Sprintf("%d %s", cfg.MaxParallel,
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

	view := widget.NewSelect([]string{
		config.ViewGrid.Label(), config.ViewList.Label(), config.ViewLight.Label(),
	}, nil)
	view.SetSelected(cfg.ViewMode.Label())

	save := widget.NewButtonWithIcon(i18n.T("common.save"), nil, func() {
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
		switch view.Selected {
		case config.ViewList.Label():
			newCfg.ViewMode = config.ViewList
		case config.ViewLight.Label():
			newCfg.ViewMode = config.ViewLight
		default:
			newCfg.ViewMode = config.ViewGrid
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
	update := widget.NewButton(i18n.T("settings.self_now"), func() {
		u.runSelfUpdate()
	})

	network := container.NewVBox(
		labelText(i18n.T("settings.network"), true),
		rowOf(i18n.T("settings.limit"), speed),
		rowOf(i18n.T("settings.parallel"), container.NewHBox(parallel, parallelValue)),
		rowOf(i18n.T("download.mode"), mode),
		rowOf("", verify),
		rowOf(i18n.T("settings.token"), token),
	)

	channels := container.NewVBox(
		labelText(i18n.T("settings.channels"), true),
		stable, edge,
	)

	behaviour := container.NewVBox(
		labelText(i18n.T("settings.behaviour"), true),
		deleteOld, closeTray, startMin,
	)

	appearance := container.NewVBox(
		labelText(i18n.T("settings.language"), true),
		lang,
		rowOf(i18n.T("settings.view_mode"), view),
	)

	install := container.NewVBox(
		labelText(i18n.T("settings.install"), true),
		container.NewHBox(shortcutBtn, openData, openLog),
		update,
	)

	body := container.NewVBox(
		rowOf(i18n.T("settings.folder"), folder),
		network, channels, behaviour, appearance, install,
	)
	// Кнопка сохранения липнет к низу: её видно всегда, даже если настроек
	// больше, чем помещается в окно.
	return container.NewBorder(nil, container.NewHBox(save), nil, nil, container.NewScroll(body))
}

// rowOf собирает строку «подпись — значение».
func rowOf(label string, value fyne.CanvasObject) fyne.CanvasObject {
	text := widget.NewLabel(label)
	text.TextStyle = fyne.TextStyle{}
	return container.NewBorder(nil, nil, text, nil, value)
}

// runSelfUpdate скачивает новую версию программы с GitHub.
func (u *UI) runSelfUpdate() {
	cfg := u.mgr.Config()
	if cfg.SelfUpdateRepo == "" {
		u.st.SetNotice(i18n.T("err.self_no_repo"))
		return
	}
	go func() {
		client := &http.Client{Timeout: selfupdate.Timeout}
		update, err := selfupdate.Prepare(context.Background(), client, cfg.GitHubToken, cfg.SelfUpdateRepo, Version)
		if err != nil {
			u.st.SetNotice(err.Error())
			return
		}
		u.st.SetNotice(i18n.TF("notify.selfupdate_started", map[string]string{}))
		_ = update
	}()
}
