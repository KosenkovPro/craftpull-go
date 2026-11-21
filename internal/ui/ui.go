//go:build cgo

// Оболочка интерфейса: окно, шапка, боковая панель, вкладки.
//
// Каркас повторяет Windows-версию: тёмная шапка со значком программы и
// статусом, боковая панель навигации со служебной подписью внизу и рабочая
// область, в которой живёт содержимое раздела.
package ui

import (
	"context"
	"fmt"
	"image/color"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/KosenkovPro/craftpull-go/internal/app"
	"github.com/KosenkovPro/craftpull-go/internal/buildinfo"
	"github.com/KosenkovPro/craftpull-go/internal/config"
	"github.com/KosenkovPro/craftpull-go/internal/i18n"
	"github.com/KosenkovPro/craftpull-go/internal/manager"
	"github.com/KosenkovPro/craftpull-go/internal/model"
	"github.com/KosenkovPro/craftpull-go/internal/selfupdate"
)

// UI — состояние окна приложения.
type UI struct {
	mgr *manager.Manager
	st  *app.State

	win       fyne.Window
	root      *fyne.Container
	body      *container.Split
	host      *fyne.Container
	content   fyne.CanvasObject
	status    *canvas.Text
	statusBox *canvas.Rectangle
	notice    *widget.Label
	search    *widget.Entry
	license   *tipLabel
	navBtns   map[app.Tab]*widget.Button
	selected  map[string]bool

	// selfMu защищает состояние обновления программы: к нему обращаются
	// и поток интерфейса, и фоновые проверки.
	selfMu sync.Mutex
	// selfUpdate — найденное обновление (nil, если обновлений нет).
	selfUpdate *selfupdate.Update
	// selfBusy — идёт проверка или установка обновления.
	selfBusy bool

	// trayRefresh пересобирает меню трея (подписи зависят от языка).
	trayRefresh func()
}

// SetSelfUpdate запоминает найденное обновление.
func (u *UI) SetSelfUpdate(upd *selfupdate.Update) {
	u.selfMu.Lock()
	u.selfUpdate = upd
	u.selfMu.Unlock()
}

// SelfUpdate возвращает найденное обновление.
func (u *UI) SelfUpdate() *selfupdate.Update {
	u.selfMu.Lock()
	defer u.selfMu.Unlock()
	return u.selfUpdate
}

// SetSelfBusy отмечает, что обновление в работе.
func (u *UI) SetSelfBusy(busy bool) {
	u.selfMu.Lock()
	u.selfBusy = busy
	u.selfMu.Unlock()
}

// SelfBusy сообщает, идёт ли обновление.
func (u *UI) SelfBusy() bool {
	u.selfMu.Lock()
	defer u.selfMu.Unlock()
	return u.selfBusy
}

// SetTrayRefresh задаёт функцию пересборки меню трея.
func (u *UI) SetTrayRefresh(f func()) { u.trayRefresh = f }

// NewUI создаёт интерфейс поверх менеджера.
func NewUI(mgr *manager.Manager, st *app.State) *UI {
	return &UI{
		mgr:      mgr,
		st:       st,
		navBtns:  make(map[app.Tab]*widget.Button),
		selected: make(map[string]bool),
	}
}

// Build собирает окно и возвращает корневой объект.
func (u *UI) Build(win fyne.Window) fyne.CanvasObject {
	applyTheme()
	u.win = win

	u.status = canvas.NewText(i18n.T("status.ready"), colDim)
	u.status.TextSize = 12
	u.statusBox = canvas.NewRectangle(colCard)
	u.statusBox.StrokeColor = colBorder
	u.statusBox.StrokeWidth = 1
	u.statusBox.CornerRadius = 8

	u.notice = widget.NewLabel("")
	u.notice.Importance = widget.LowImportance
	u.notice.SizeName = theme.SizeNameCaptionText

	// Лицензия с пояснением по наведению.
	u.license = newTipLabel(License, i18n.T("license.hint"))
	u.license.label.Importance = widget.HighImportance

	u.search = widget.NewEntry()
	u.search.SetPlaceHolder(i18n.T("repo.slug_hint"))
	u.search.OnChanged = func(text string) { u.Rebuild() }

	header := u.buildHeader()
	sidebar := u.buildSidebar()
	// host — постоянный контейнер рабочей области. Содержимое раздела
	// меняется целиком, а сам раздел остаётся: Fyne запоминает объекты
	// раздева при создании и подмену содержимого уже не рисует.
	u.host = container.New(layout.NewMaxLayout())
	u.body = container.NewHSplit(sidebar, u.host)
	u.body.Offset = 0.16

	// Border, а не VBox: вертикальная коробка отдаёт лишнее место последнему
	// элементу, и рабочая область оставалась бы в пару пикселей высотой.
	u.root = container.NewBorder(header, nil, nil, nil, u.body)
	u.Rebuild()
	return u.root
}

// buildHeader собирает верхнюю панель: значок, название, статус и действия.
func (u *UI) buildHeader() fyne.CanvasObject {
	logo := programIcon(26)

	title := widget.NewLabelWithStyle(buildinfo.Name, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	title.SizeName = theme.SizeNameSubHeadingText

	subtitle := labelDim(i18n.T("app.subtitle"))
	subtitle.SizeName = theme.SizeNameCaptionText

	status := container.NewStack(u.statusBox, container.NewPadded(center(u.status)))

	check := widget.NewButton(i18n.T("common.refresh"), func() {
		go u.mgr.RefreshAll(context.Background(), true)
	})
	download := widget.NewButton(i18n.T("common.download_new"), func() {
		go u.mgr.DownloadAll(context.Background())
	})
	pause := widget.NewButton(i18n.T("common.pause"), func() {
		u.mgr.TogglePause()
		u.Rebuild()
	})
	stop := widget.NewButton(i18n.T("common.stop"), func() {
		u.mgr.StopAll()
	})

	langLabel := "EN"
	if u.st.Lang() == i18n.RU {
		langLabel = "RU"
	}
	lang := widget.NewButton(langLabel, func() {
		if u.st.Lang() == i18n.RU {
			u.st.SetLang(i18n.EN)
		} else {
			u.st.SetLang(i18n.RU)
		}
		u.Rebuild()
		if u.trayRefresh != nil {
			u.trayRefresh()
		}
	})

	left := container.NewHBox(logo, title, subtitle, status)
	right := container.NewHBox(check, download, pause, stop, lang)
	return container.NewBorder(nil, nil, left, right)
}

// buildSidebar собирает боковую панель навигации.
//
// Внизу — служебная подпись: версия, автор и режим работы в фоне,
// как в Windows-версии.
func (u *UI) buildSidebar() fyne.CanvasObject {
	makeButton := func(tab app.Tab, label string) *widget.Button {
		btn := widget.NewButton(label, func() {
			u.st.SetTab(tab)
			u.Rebuild()
		})
		u.navBtns[tab] = btn
		return btn
	}
	box := container.NewVBox(
		makeButton(app.TabRepos, i18n.T("nav.repos")),
		makeButton(app.TabSettings, i18n.T("nav.settings")),
		makeButton(app.TabLogs, i18n.T("nav.logs")),
	)
	box.Objects[0].(*widget.Button).Importance = widget.HighImportance

	footer := container.NewVBox(
		labelDim(i18n.T("nav.footer")),
		labelDim(fmt.Sprintf("%s %s", i18n.T("nav.version"), Version)),
		// Лицензия программы: при наведении появляется объяснение,
		// что она разрешает.
		u.license,
		labelDim(i18n.T("app.author")),
		u.notice,
	)
	return container.NewBorder(container.NewVBox(container.NewPadded(box)), container.NewPadded(footer), nil, nil, nil)
}

// Rebuild перестраивает содержимое окна.
func (u *UI) Rebuild() {
	if u.root == nil {
		return
	}
	for tab, btn := range u.navBtns {
		if tab == u.st.Tab() {
			btn.Importance = widget.HighImportance
		} else {
			btn.Importance = widget.MediumImportance
		}
		btn.Refresh()
	}
	u.status.Text = u.statusText()
	u.status.Color = u.statusColor()
	u.statusBox.StrokeColor = u.statusColor()
	canvas.Refresh(u.statusBox)
	u.notice.SetText(u.st.Notice())

	switch u.st.Tab() {
	case app.TabSettings:
		u.content = u.buildSettings()
	case app.TabLogs:
		u.content = u.buildLogs()
	default:
		u.content = u.buildRepos()
	}

	// Панель релизов живёт справа от содержимого и не прокручивается вместе с ним.
	main := container.New(layout.NewMaxLayout())
	main.Add(u.content)
	if open, slug := u.st.ReleasesOpen(); open {
		panel := u.buildReleases(slug)
		u.host.Objects = []fyne.CanvasObject{container.NewHSplit(main, panel)}
		u.body.SetOffset(0.62)
	} else {
		u.host.Objects = []fyne.CanvasObject{main}
		u.body.SetOffset(0.16)
	}
	u.host.Refresh()
	canvas.Refresh(u.root)
}

// statusColor возвращает цвет чипа статуса: зелёный — идёт загрузка,
// красный — есть ошибки, серый — программа ждёт.
func (u *UI) statusColor() color.Color {
	if u.mgr.HasErrors() {
		return colError
	}
	for _, t := range u.mgr.Tasks().All() {
		if !t.State.IsFinished() {
			return colOK
		}
	}
	return colDim
}

// statusText собирает текст чипа статуса.
func (u *UI) statusText() string {
	tasks := u.mgr.Tasks().All()
	active := 0
	var speed float64
	var downloaded, total uint64
	for _, t := range tasks {
		if t.State.IsFinished() {
			continue
		}
		active++
		speed += t.Speed
		downloaded += t.Downloaded
		total += t.Total
	}
	checks := u.mgr.Checks()
	if checks.Running {
		return i18n.TF("status.checking", map[string]string{
			"{n}": fmt.Sprintf("%d", checks.Done()),
			"{m}": fmt.Sprintf("%d", checks.Total()),
		})
	}
	if active == 0 {
		return i18n.T("status.ready")
	}
	return fmt.Sprintf("%s · %s · %s / %s",
		i18n.T("status.downloading"),
		model.HumanSpeed(speed),
		model.HumanSize(downloaded),
		model.HumanSize(total))
}

// Tick обновляет интерфейс по таймеру: прогресс, очередь, журнал.
func (u *UI) Tick() {
	u.Rebuild()
}

// filteredRepos возвращает репозитории, отобранные по строке поиска.
func (u *UI) filteredRepos() []config.RepoConfig {
	cfg := u.mgr.Config()
	query := strings.ToLower(strings.TrimSpace(u.search.Text))
	out := make([]config.RepoConfig, 0, len(cfg.Repos))
	for _, r := range cfg.Repos {
		if query == "" ||
			strings.Contains(strings.ToLower(r.Slug), query) ||
			strings.Contains(strings.ToLower(r.Description), query) {
			out = append(out, r)
		}
	}
	return out
}

// groupedRepos раскладывает репозитории по категориям программ.
//
// Порядок категорий задан в config.CategoryOrder: сначала свои программы,
// потом готовый каталог, в конце — всё, что добавил пользователь сам.
func (u *UI) groupedRepos() ([]string, map[string][]config.RepoConfig) {
	byCat := make(map[string][]config.RepoConfig)
	for _, r := range u.filteredRepos() {
		key := r.Category
		if key == "" {
			key = config.CatOther
		}
		byCat[key] = append(byCat[key], r)
	}
	cats := make([]string, 0, len(byCat))
	for _, key := range config.CategoryOrder {
		if _, ok := byCat[key]; ok {
			cats = append(cats, key)
		}
	}
	// Категории, которых нет в списке (пользователь придумал свою), — в конец.
	for key := range byCat {
		known := false
		for _, c := range cats {
			if c == key {
				known = true
				break
			}
		}
		if !known {
			cats = append(cats, key)
		}
	}
	return cats, byCat
}

// taskForRepo ищет активную задачу загрузки репозитория.
func (u *UI) taskForRepo(slug string) (model.TaskProgress, bool) {
	for _, t := range u.mgr.Tasks().All() {
		if t.Repo == slug && !t.State.IsFinished() {
			return t, true
		}
	}
	return model.TaskProgress{}, false
}
