//go:build cgo

// Список репозиториев: сетка, список и режим «без картинок».
package ui

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/KosenkovPro/craftpull-go/internal/app"
	"github.com/KosenkovPro/craftpull-go/internal/config"
	"github.com/KosenkovPro/craftpull-go/internal/i18n"
	"github.com/KosenkovPro/craftpull-go/internal/manager"
	"github.com/KosenkovPro/craftpull-go/internal/model"
)

// buildRepos собирает вкладку со списком репозиториев.
func (u *UI) buildRepos() fyne.CanvasObject {
	toolbar := u.buildReposToolbar()
	body := container.NewVBox()
	authors, byAuthor := u.groupedRepos()
	for _, author := range authors {
		body.Add(labelText(i18n.T("repo.group")+" · "+author, true))
		switch u.st.View() {
		case config.ViewGrid:
			body.Add(u.gridFor(byAuthor[author]))
		case config.ViewLight:
			body.Add(u.lightFor(byAuthor[author]))
		default:
			for _, rc := range byAuthor[author] {
				body.Add(u.repoCard(rc, false))
			}
		}
	}
	if len(authors) == 0 {
		body.Add(widget.NewLabel(i18n.T("repo.no_releases")))
	}
	return container.NewBorder(toolbar, nil, nil, nil, container.NewScroll(body))
}

// buildReposToolbar собирает панель инструментов над списком.
func (u *UI) buildReposToolbar() fyne.CanvasObject {
	add := widget.NewButton(i18n.T("repo.add"), func() { u.promptAddRepo() })
	selectAll := widget.NewButton(i18n.T("repo.select_all"), func() {
		cfg := u.mgr.Config()
		for i := range cfg.Repos {
			cfg.Repos[i].Enabled = true
		}
		_ = u.mgr.SaveConfig(cfg)
		u.Rebuild()
	})
	selectNone := widget.NewButton(i18n.T("repo.select_none"), func() {
		cfg := u.mgr.Config()
		for i := range cfg.Repos {
			cfg.Repos[i].Enabled = false
		}
		_ = u.mgr.SaveConfig(cfg)
		u.Rebuild()
	})

	modeButtons := container.NewHBox()
	for _, mode := range []config.ViewMode{config.ViewGrid, config.ViewList, config.ViewLight} {
		mode := mode
		btn := widget.NewButton(mode.Label(), func() {
			cfg := u.mgr.Config()
			cfg.ViewMode = mode
			_ = u.mgr.SaveConfig(cfg)
			u.st.SetView(mode)
			u.Rebuild()
		})
		if mode == u.st.View() {
			btn.Importance = widget.HighImportance
		}
		modeButtons.Add(btn)
	}

	left := container.NewHBox(add, selectAll, selectNone)
	// Поле поиска занимает отдельную строку: так его ширина не зависит от
	// длины подсказки и в него удобно вводить «владелец/репозиторий».
	searchRow := container.NewBorder(nil, nil, nil, nil, u.search)
	buttonRow := container.NewBorder(nil, nil, left, modeButtons)
	return container.NewVBox(searchRow, buttonRow)
}

// gridFor строит сетку карточек репозиториев.
func (u *UI) gridFor(repos []config.RepoConfig) fyne.CanvasObject {
	grid := container.NewGridWithColumns(3)
	for _, rc := range repos {
		grid.Add(u.repoCard(rc, true))
	}
	return grid
}

// lightFor строит строки без картинок: имя, метаданные, прогресс.
func (u *UI) lightFor(repos []config.RepoConfig) fyne.CanvasObject {
	rows := container.NewVBox()
	for _, rc := range repos {
		rows.Add(u.lightRow(rc))
	}
	return rows
}

// lightRow — строка режима «без картинок».
func (u *UI) lightRow(rc config.RepoConfig) fyne.CanvasObject {
	rt, _ := u.mgr.RepoRuntime(rc.Slug)

	name := container.NewVBox(labelText(rc.ShortName(), true), widget.NewLabel(rc.Slug))

	meta := container.NewVBox(widget.NewLabel(versionLine(rt)))
	meta.Objects[0].(*widget.Label).Wrapping = fyne.TextWrapOff
	if rc.Description != "" {
		desc := widget.NewLabel(rc.Description)
		desc.Wrapping = fyne.TextWrapWord
		meta.Add(desc)
	}

	left := container.NewHSplit(name, meta)
	left.Offset = 0.22

	// Место под прогресс нужно только во время загрузки: иначе правая
	// колонка остаётся пустой и занимает треть окна.
	task, ok := u.taskForRepo(rc.Slug)
	if !ok {
		return left
	}
	progress := container.NewVBox(progressBlock(task))
	row := container.NewHSplit(left, progress)
	row.Offset = 0.7
	return row
}

// repoCard собирает карточку репозитория.
//
// В сетке карточка компактная: у скачиваемого репозитория остаётся только
// полоса с процентом. В списке и в режиме «без картинок» — подробный блок.
func (u *UI) repoCard(rc config.RepoConfig, compact bool) fyne.CanvasObject {
	cfg := u.mgr.Config()
	rt, _ := u.mgr.RepoRuntime(rc.Slug)

	head := container.NewHBox(labelText(rc.ShortName(), true))
	if rt.LatestStable != nil {
		head.Add(chip(rt.LatestStable.Tag, colOK))
	} else if rt.LatestEdge != nil {
		head.Add(chip(rt.LatestEdge.Tag, colEdge))
	} else {
		head.Add(chip("—", colDim))
	}
	head.Add(layout.NewSpacer())
	head.Add(widget.NewLabel(i18n.TF("repo.selected_count", map[string]string{
		"{n}": fmt.Sprintf("%d", countEnabled(cfg)),
		"{m}": fmt.Sprintf("%d", len(cfg.Repos)),
	})))

	sub := widget.NewLabel(rc.Slug)
	sub.TextStyle = fyne.TextStyle{}
	sub.Wrapping = fyne.TextWrapOff

	desc := widget.NewLabel(rc.Description)
	desc.Wrapping = fyne.TextWrapWord
	desc.TextStyle = fyne.TextStyle{}

	meta := container.NewHBox()
	if rt.LastCheck != nil {
		meta.Add(chip(checkedLabel(*rt.LastCheck), colDim))
	}
	if len(rc.Pinned) > 0 {
		meta.Add(chip(i18n.TF("repo.pinned_n", map[string]string{"{n}": fmt.Sprintf("%d", len(rc.Pinned))}), colWarn))
	}

	filter := u.filterSelect(rc, compact)
	actions := u.repoActions(rc)

	content := container.NewVBox(head, sub)
	if rc.Description != "" {
		content.Add(desc)
	}
	content.Add(meta)
	content.Add(filter)
	content.Add(actions)

	// Прогресс: в сетке — только полоса с процентом, в списке — подробный блок.
	if task, ok := u.taskForRepo(rc.Slug); ok {
		if compact {
			fraction := task.Fraction()
			bar := newProgressBar(float64(fraction), fmt.Sprintf("%d %%", int(fraction*100)))
			content.Add(bar)
		} else {
			content.Add(progressBlock(task))
		}
	}

	card := container.NewStack(canvas.NewRectangle(colCard), container.NewPadded(content))
	return card
}

// versionLine собирает строку с версией репозитория и каналом релиза.
func versionLine(rt manager.RepoRuntime) string {
	switch {
	case rt.LatestStable != nil:
		return rt.LatestStable.Tag + " · " + i18n.T("popup.stable")
	case rt.LatestEdge != nil:
		return rt.LatestEdge.Tag + " · " + i18n.T("popup.edge")
	default:
		return "—"
	}
}

// checkedLabel собирает подпись «проверено 5 мин назад».
func checkedLabel(raw string) string {
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return i18n.T("repo.checked") + " " + model.HumanDate(raw)
	}
	mins := int(time.Since(t).Minutes())
	switch {
	case mins < 1:
		return i18n.T("repo.checked") + " " + i18n.T("time.now")
	case mins < 60:
		return i18n.T("repo.checked") + " " + i18n.TF("time.minutes",
			map[string]string{"{n}": fmt.Sprintf("%d", mins)})
	case mins < 24*60:
		return i18n.T("repo.checked") + " " + i18n.TF("time.hours",
			map[string]string{"{n}": fmt.Sprintf("%d", mins/60)})
	default:
		return i18n.T("repo.checked") + " " + model.HumanDate(raw)
	}
}

// filterSelect собирает выпадающий список файлов релиза.
//
// В сетке места мало, поэтому подписи короткие: «Windows MSI» вместо
// «Windows MSI (установщик)» — иначе список вылезает за края карточки.
func (u *UI) filterSelect(rc config.RepoConfig, compact bool) fyne.CanvasObject {
	rt, ok := u.mgr.RepoRuntime(rc.Slug)
	if !ok || len(rt.Releases) == 0 {
		return widget.NewLabel(i18n.T("repo.filter") + ": —")
	}
	options := manager.AssetOptions(rc.Slug, &rt.Releases[0])
	if len(options) == 0 {
		return widget.NewLabel(i18n.T("repo.filter") + ": " + i18n.T("filter.source"))
	}
	label := func(k config.AssetKind) string {
		if compact {
			return k.ShortLabel()
		}
		return k.Label()
	}
	// Подписи и виды файлов идут парами: по подписи выбирается вид.
	kinds := make([]config.AssetKind, 0, len(options)+1)
	labels := make([]string, 0, len(options)+1)
	kinds = append(kinds, config.AssetAuto)
	labels = append(labels, label(config.AssetAuto))
	for _, opt := range options {
		kinds = append(kinds, opt.Kind)
		labels = append(labels, label(opt.Kind))
	}
	selected := 0
	for i, opt := range options {
		if opt.Kind == rc.Filter {
			selected = i + 1
			break
		}
	}
	// SetSelected вызывает обработчик, поэтому на время установки значения
	// он молчит: иначе перерисовка списка каждые 700 мс переписывала бы
	// файл настроек.
	applying := true
	combo := widget.NewSelect(labels, func(choice string) {
		if applying {
			return
		}
		cfg := u.mgr.Config()
		target := cfg.Repo(rc.Slug)
		if target == nil {
			return
		}
		for i, l := range labels {
			if l == choice {
				target.Filter = kinds[i]
				break
			}
		}
		_ = u.mgr.SaveConfig(cfg)
	})
	if selected < len(labels) {
		combo.SetSelected(labels[selected])
	}
	applying = false
	row := container.NewBorder(nil, nil, widget.NewLabel(i18n.T("repo.filter")+": "), nil, combo)
	return row
}

// repoActions собирает кнопки карточки.
func (u *UI) repoActions(rc config.RepoConfig) fyne.CanvasObject {
	download := widget.NewButton(i18n.T("common.download"), func() {
		go u.mgr.DownloadRepo(context.Background(), rc.Slug, manager.DownloadRequest{Targets: true})
	})
	releases := widget.NewButton(i18n.T("common.releases"), func() {
		u.st.OpenReleases(rc.Slug)
		u.Rebuild()
	})
	pin := widget.NewButton(i18n.T("common.pin"), func() {
		u.togglePin(rc.Slug)
	})
	open := widget.NewButton(i18n.T("hero.open_folder"), func() {
		u.openRepoFolder(rc.Slug)
	})
	return container.NewHBox(download, releases, pin, open)
}

// togglePin закрепляет или открепляет последний стабильный релиз.
func (u *UI) togglePin(slug string) {
	cfg := u.mgr.Config()
	target := cfg.Repo(slug)
	if target == nil {
		return
	}
	rt, ok := u.mgr.RepoRuntime(slug)
	if !ok || rt.LatestStable == nil {
		return
	}
	tag := rt.LatestStable.Tag
	pinned := false
	for _, p := range target.Pinned {
		if p == tag {
			pinned = true
			break
		}
	}
	if pinned {
		kept := make([]string, 0, len(target.Pinned))
		for _, p := range target.Pinned {
			if p != tag {
				kept = append(kept, p)
			}
		}
		target.Pinned = kept
	} else {
		target.Pinned = append(target.Pinned, tag)
	}
	_ = u.mgr.SaveConfig(cfg)
	u.Rebuild()
}

// openRepoFolder открывает папку скачанной версии.
func (u *UI) openRepoFolder(slug string) {
	rt, ok := u.mgr.RepoRuntime(slug)
	if !ok || len(rt.Installed) == 0 {
		return
	}
	dir := rt.Installed[0].Dir
	if runtime.GOOS == "windows" {
		_ = exec.Command("explorer", dir).Start()
		return
	}
	_ = exec.Command("open", dir).Start()
}

// promptAddRepo просит имя нового репозитория.
func (u *UI) promptAddRepo() {
	entry := widget.NewEntry()
	entry.SetPlaceHolder(i18n.T("repo.slug_hint"))
	showDialog(u.win, i18n.T("repo.add"), entry, func() {
		slug := app.NormalizeSlug(entry.Text)
		if !app.ValidSlug(slug) {
			return
		}
		cfg := u.mgr.Config()
		if cfg.Repo(slug) != nil {
			return
		}
		rc := config.NewRepoConfig(slug)
		rc.Enabled = true
		cfg.Repos = append(cfg.Repos, rc)
		_ = u.mgr.SaveConfig(cfg)
		u.Rebuild()
	})
}

// countEnabled считает отмеченные репозитории.
func countEnabled(cfg config.Config) int {
	n := 0
	for _, r := range cfg.Repos {
		if r.Enabled {
			n++
		}
	}
	return n
}
