//go:build cgo

// Список репозиториев: сетка, список и режим «без картинок».
//
// Оформление повторяет Windows-версию: в сетке — компактные плитки со
// значком и версией, раскрытая карточка-герой — во всю ширину, в списке —
// подробные карточки с блоком «На диске», в режиме «без картинок» — строки
// с прогрессом загрузки.
package ui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/KosenkovPro/craftpull-go/internal/app"
	"github.com/KosenkovPro/craftpull-go/internal/config"
	"github.com/KosenkovPro/craftpull-go/internal/i18n"
	"github.com/KosenkovPro/craftpull-go/internal/manager"
	"github.com/KosenkovPro/craftpull-go/internal/model"
)

// buildRepos собирает страницу репозиториев.
func (u *UI) buildRepos() fyne.CanvasObject {
	cfg := u.mgr.Config()

	// Шапка страницы: название, счётчик выбранных и импорт/экспорт списка.
	title := heading(i18n.T("nav.repos"))
	counter := labelDim(i18n.TF("repo.selected_count", map[string]string{
		"{n}": fmt.Sprintf("%d", countEnabled(cfg)),
		"{m}": fmt.Sprintf("%d", len(cfg.Repos)),
	}))
	importBtn := widget.NewButton(i18n.T("repo.import"), func() { u.promptImport() })
	exportBtn := widget.NewButton(i18n.T("repo.export"), func() { u.promptExport() })
	head := container.NewBorder(nil, nil,
		container.NewHBox(title, counter),
		container.NewHBox(importBtn, exportBtn),
	)

	body := container.NewVBox()
	authors, byAuthor := u.groupedRepos()
	for _, author := range authors {
		body.Add(u.groupHeader(author, len(byAuthor[author])))
		repos := byAuthor[author]
		// Раскрытая карточка стоит на месте своей плитки.
		hero := u.st.Hero()
		for i, rc := range repos {
			if rc.Slug != hero {
				continue
			}
			body.Add(u.heroCard(rc))
			repos = append(repos[:i:i], repos[i+1:]...)
			break
		}
		switch u.st.View() {
		case config.ViewGrid:
			body.Add(u.gridFor(repos))
		case config.ViewLight:
			body.Add(u.lightFor(repos))
		default:
			for _, rc := range repos {
				body.Add(u.listCard(rc))
			}
		}
	}
	if len(authors) == 0 {
		body.Add(labelDim(i18n.T("repo.no_releases")))
	}

	top := container.NewVBox(head, u.buildReposToolbar())
	return container.NewBorder(top, nil, nil, nil, container.NewScroll(body))
}

// groupHeader собирает заголовок группы репозиториев одного автора.
func (u *UI) groupHeader(author string, count int) fyne.CanvasObject {
	return container.NewHBox(
		labelDim(i18n.T("repo.group")),
		labelAccent(author, false),
		labelDim(fmt.Sprintf("%d", count)),
	)
}

// buildReposToolbar собирает панель над списком: поиск, действия и режим вида.
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
	check := widget.NewButton(i18n.T("common.refresh"), func() {
		go u.mgr.RefreshAll(context.Background(), true)
		u.Rebuild()
	})

	// Поле поиска занимает отдельную строку: так его ширина не зависит от
	// длины подсказки и в него удобно вводить «владелец/репозиторий».
	searchRow := container.NewBorder(nil, nil, container.New(widthLayout{width: 300}, u.search), nil, nil)
	buttonRow := container.NewBorder(nil, nil,
		container.NewHBox(add, selectAll, selectNone),
		nil,
	)
	modeRow := container.NewBorder(nil, nil,
		container.NewHBox(labelDim(i18n.T("repo.view_mode")), modeButtons),
		check,
	)
	return container.NewVBox(searchRow, buttonRow, modeRow)
}

// widthLayout — объект заданной ширины во всю высоту контейнера.
type widthLayout struct{ width float32 }

func (l widthLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objs {
		o.Resize(fyne.NewSize(l.width, size.Height))
		o.Move(fyne.NewPos(0, 0))
	}
}

func (l widthLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	if len(objs) == 0 {
		return fyne.NewSize(l.width, 0)
	}
	return fyne.NewSize(l.width, objs[0].MinSize().Height)
}

// gridFor строит сетку плиток репозиториев.
func (u *UI) gridFor(repos []config.RepoConfig) fyne.CanvasObject {
	grid := container.NewGridWithColumns(6)
	for _, rc := range repos {
		grid.Add(u.gridTile(rc))
	}
	return grid
}

// tileLayout — плитка сетки: фон, содержимое и галочка в правом верхнем углу.
type tileLayout struct{}

func (tileLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	for i, o := range objs {
		if i == len(objs)-1 && len(objs) > 2 {
			// Последний объект — галочка выделения в правом верхнем углу.
			s := o.MinSize()
			o.Resize(s)
			o.Move(fyne.NewPos(size.Width-s.Width-12, 10))
			continue
		}
		if i == 1 {
			// Содержимое плитки стоит по центру.
			s := o.MinSize()
			o.Resize(s)
			o.Move(fyne.NewPos((size.Width-s.Width)/2, (size.Height-s.Height)/2))
			continue
		}
		o.Resize(size)
		o.Move(fyne.NewPos(0, 0))
	}
}

func (tileLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	if len(objs) < 2 {
		return fyne.NewSize(150, 130)
	}
	s := objs[1].MinSize()
	return fyne.NewSize(s.Width+28, s.Height+24)
}

// gridTile собирает плитку репозитория: значок, имя и версия.
func (u *UI) gridTile(rc config.RepoConfig) fyne.CanvasObject {
	rt, _ := u.mgr.RepoRuntime(rc.Slug)

	version := "—"
	tint := colDim
	if rt.LatestStable != nil {
		version, tint = rt.LatestStable.Tag, colOK
	} else if rt.LatestEdge != nil {
		version, tint = rt.LatestEdge.Tag, colEdge
	}

	// Значок — картинка, нажатие раскрывает карточку репозитория.
	open := newTappableImage(repoIconImage(rc.Icon, 56), func() {
		if u.st.Hero() == rc.Slug {
			u.st.SetHero("")
		} else {
			u.st.SetHero(rc.Slug)
		}
		u.Rebuild()
	})
	more := newTappableBox(fyne.NewSize(14, 14), chevronGlyph(14, colDim), func() {
		u.st.OpenReleases(rc.Slug)
		u.Rebuild()
	})

	iconBox := container.New(cornerLayout{offsetX: 2, offsetY: -3}, open, more)

	name := labelText(rc.ShortName(), false)
	name.Alignment = fyne.TextAlignCenter
	name.Truncation = fyne.TextTruncateEllipsis

	ver := textColored(version, tint, 12, false)
	ver.(*canvas.Text).Alignment = fyne.TextAlignCenter

	content := container.NewVBox(
		container.NewCenter(iconBox),
		container.New(widthLayout{width: 150}, name),
		container.NewCenter(ver),
	)

	bg := canvas.NewRectangle(colCard)
	bg.StrokeColor = colBorder
	bg.StrokeWidth = 1
	bg.CornerRadius = 12

	objs := []fyne.CanvasObject{bg, content}
	if rc.Enabled {
		objs = append(objs, tick(14))
	}
	return container.New(tileLayout{}, objs...)
}

// heroCard собирает раскрытую карточку репозитория — как в Windows-версии.
func (u *UI) heroCard(rc config.RepoConfig) fyne.CanvasObject {
	rt, _ := u.mgr.RepoRuntime(rc.Slug)

	name := widget.NewLabelWithStyle(rc.ShortName(), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	name.SizeName = theme.SizeNameSubHeadingText
	head := container.NewHBox(name)
	if len(rc.Pinned) > 0 {
		head.Add(labelWarn(i18n.TF("repo.pinned_n", map[string]string{
			"{n}": fmt.Sprintf("%d", len(rc.Pinned)),
		}), false))
	}

	meta := labelDim(u.repoMeta(rc, rt))

	badge := container.NewHBox()
	if rt.LatestStable != nil {
		badge.Add(pill(i18n.T("popup.stable")+" "+rt.LatestStable.Tag, colOK))
	} else if rt.LatestEdge != nil {
		badge.Add(pill(i18n.T("popup.edge")+" "+rt.LatestEdge.Tag, colEdge))
	}

	info := container.NewVBox(head, meta, badge)
	if rc.Description != "" {
		desc := widget.NewLabel(rc.Description)
		desc.Wrapping = fyne.TextWrapWord
		info.Add(desc)
	}
	if sizes := u.installedSizes(rt); sizes != "" {
		info.Add(labelDim(sizes))
	}

	left := container.NewBorder(nil, nil, container.NewWithoutLayout(u.repoIcon(rc, 56)), nil, info)
	actions := u.repoActions(rc)
	files := container.NewBorder(nil, nil, labelDim(i18n.T("repo.filter")+":"), nil,
		container.New(widthLayout{width: 250}, u.filterSelect(rc, false)))

	rows := container.NewVBox(left,
		container.NewBorder(nil, nil, container.NewWithoutLayout(actions), files))

	closeBtn := widget.NewButtonWithIcon("", theme.CancelIcon(), func() {
		u.st.SetHero("")
		u.Rebuild()
	})
	closeBtn.Importance = widget.LowImportance
	topRight := container.NewHBox(closeBtn)
	if rc.Enabled {
		topRight.Add(tick(14))
	}

	return cardWithBorder(container.NewBorder(nil, nil, nil, topRight, rows), colAccent)
}

// listCard собирает подробную карточку репозитория для режима «Список».
func (u *UI) listCard(rc config.RepoConfig) fyne.CanvasObject {
	rt, _ := u.mgr.RepoRuntime(rc.Slug)

	name := widget.NewLabelWithStyle(rc.ShortName(), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	name.SizeName = theme.SizeNameSubHeadingText
	head := container.NewHBox(name)
	if len(rc.Pinned) > 0 {
		head.Add(labelWarn(i18n.TF("repo.pinned_n", map[string]string{
			"{n}": fmt.Sprintf("%d", len(rc.Pinned)),
		}), false))
	}

	slug := labelDim(rc.Slug)
	desc := widget.NewLabel(rc.Description)
	desc.Wrapping = fyne.TextWrapWord

	badge := container.NewHBox()
	if rt.LatestStable != nil {
		badge.Add(pill(i18n.T("popup.stable")+" "+rt.LatestStable.Tag, colOK))
	} else if rt.LatestEdge != nil {
		badge.Add(pill(i18n.T("popup.edge")+" "+rt.LatestEdge.Tag, colEdge))
	}
	if rt.LastCheck != nil {
		badge.Add(labelDim(i18n.T("repo.checked")+": "+model.HumanDate(*rt.LastCheck)))
	}

	left := container.NewVBox(head, slug, desc, badge)
	if block := u.installedBlock(rc, rt); block != nil {
		left.Add(block)
	}
	files := container.NewBorder(nil, nil, labelDim(i18n.T("repo.filter")+":"), nil,
		container.New(widthLayout{width: 250}, u.filterSelect(rc, false)))

	// Значок слева, сведения по центру, кнопки справа — как в Windows-версии.
	row := container.NewBorder(nil, nil, container.NewWithoutLayout(u.repoIcon(rc, 40)),
		container.NewWithoutLayout(u.repoActions(rc)), left)
	return card(container.NewVBox(row, files))
}

// installedBlock собирает блок «На диске» с уже скачанными версиями.
func (u *UI) installedBlock(rc config.RepoConfig, rt manager.RepoRuntime) fyne.CanvasObject {
	if len(rt.Installed) == 0 {
		return nil
	}
	rows := container.NewVBox(labelDim(i18n.T("repo.on_disk")))
	for _, v := range rt.Installed {
		pin := widget.NewButton(i18n.T("common.pin"), func() { u.pinVersion(rc.Slug, v.Tag) })
		pin.Importance = widget.LowImportance
		del := widget.NewButton(i18n.T("common.delete"), func() { u.removeVersion(rc.Slug, v.Tag) })
		del.Importance = widget.LowImportance
		line := container.NewHBox(
			labelWarn(fmt.Sprintf("%s · %s", v.Tag, model.HumanSize(v.Size)), false),
			labelDim(fmt.Sprintf("%d %s", len(v.Files), i18n.T("common.files"))),
			pin,
			del,
		)
		rows.Add(line)
	}
	rect := canvas.NewRectangle(colCardSoft)
	rect.CornerRadius = 8
	return container.NewStack(rect, container.NewPadded(rows))
}

// installedSizes собирает строку с размерами скачанных версий.
func (u *UI) installedSizes(rt manager.RepoRuntime) string {
	if len(rt.Installed) == 0 {
		return ""
	}
	parts := make([]string, 0, len(rt.Installed))
	for _, v := range rt.Installed {
		parts = append(parts, fmt.Sprintf("%s · %s", v.Tag, model.HumanSize(v.Size)))
	}
	return joinParts(parts)
}

// joinParts соединяет части строки через три пробела.
func joinParts(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += "   "
		}
		out += p
	}
	return out
}

// lightFor строит строки режима «без картинок».
func (u *UI) lightFor(repos []config.RepoConfig) fyne.CanvasObject {
	rows := container.NewVBox()
	if summary := u.lightSummary(); summary != nil {
		rows.Add(summary)
	}
	for _, rc := range repos {
		rows.Add(u.lightRow(rc))
	}
	return rows
}

// lightSummary собирает общую полосу загрузок над списком.
func (u *UI) lightSummary() fyne.CanvasObject {
	tasks := u.mgr.Tasks().All()
	var downloaded, total uint64
	var speed float64
	active := 0
	for _, t := range tasks {
		if t.State.IsFinished() {
			continue
		}
		active++
		downloaded += t.Downloaded
		total += t.Total
		speed += t.Speed
	}
	if active == 0 {
		return nil
	}
	fraction := float32(0)
	if total > 0 {
		fraction = float32(downloaded) / float32(total)
	}
	line := container.NewHBox(
		labelDim(fmt.Sprintf("%s: %s / %s", i18n.T("status.total"),
			model.HumanSize(downloaded), model.HumanSize(total))),
		labelAccent(model.HumanSpeed(speed), false),
	)
	bar := newProgressBar(float64(fraction), fmt.Sprintf("%d %%", int(fraction*100)))
	return card(container.NewVBox(line, bar))
}

// lightRow — строка режима «без картинок»: галочка, имя, метаданные, прогресс.
func (u *UI) lightRow(rc config.RepoConfig) fyne.CanvasObject {
	rt, _ := u.mgr.RepoRuntime(rc.Slug)

	// Галочка отмечает репозиторий для загрузки.
	// SetChecked тоже вызывает обработчик, поэтому на время установки
	// значения он молчит — иначе список перестраивался бы по кругу.
	applying := true
	check := widget.NewCheck("", func(on bool) {
		if applying {
			return
		}
		cfg := u.mgr.Config()
		target := cfg.Repo(rc.Slug)
		if target == nil {
			return
		}
		target.Enabled = on
		_ = u.mgr.SaveConfig(cfg)
		u.Rebuild()
	})
	check.SetChecked(rc.Enabled)
	applying = false

	name := container.NewVBox(
		labelText(rc.ShortName(), true),
		labelDim(rc.Slug),
	)

	meta := labelDim(u.repoMeta(rc, rt))
	meta.Wrapping = fyne.TextWrapWord
	center := container.NewVBox(meta)
	if rc.Description != "" {
		desc := widget.NewLabel(rc.Description)
		desc.Wrapping = fyne.TextWrapWord
		center.Add(desc)
	}
	if task, ok := u.taskForRepo(rc.Slug); ok {
		center.Add(progressBlock(task))
	}

	row := container.NewBorder(nil, nil,
		container.NewHBox(check, name),
		container.NewWithoutLayout(u.repoActions(rc)),
		center,
	)
	return card(row)
}

// repoMeta собирает строку сведений о репозитории: слаг, версия, даты.
func (u *UI) repoMeta(rc config.RepoConfig, rt manager.RepoRuntime) string {
	parts := []string{rc.Slug}
	if rt.LatestStable != nil {
		parts = append(parts, rt.LatestStable.Tag)
	} else if rt.LatestEdge != nil {
		parts = append(parts, rt.LatestEdge.Tag)
	}
	if rt.LatestStable != nil && rt.LatestStable.PublishedAt != nil {
		parts = append(parts, model.HumanDate(*rt.LatestStable.PublishedAt))
	}
	if rt.LastCheck != nil {
		parts = append(parts, i18n.T("repo.checked")+": "+model.HumanDate(*rt.LastCheck))
	}
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += " · "
		}
		out += p
	}
	return out
}

// repoIcon возвращает значок репозитория: собственный, если он скачан,
// иначе значок программы.
func (u *UI) repoIcon(rc config.RepoConfig, size float32) fyne.CanvasObject {
	return repoIconImage(rc.Icon, size)
}

// filterSelect собирает выпадающий список файлов релиза.
//
// В сетке места мало, поэтому подписи короткие: «Windows MSI» вместо
// «Windows MSI (установщик)» — иначе список вылезает за края карточки.
func (u *UI) filterSelect(rc config.RepoConfig, compact bool) fyne.CanvasObject {
	rt, ok := u.mgr.RepoRuntime(rc.Slug)
	if !ok || len(rt.Releases) == 0 {
		return labelDim(i18n.T("repo.filter") + ": —")
	}
	options := manager.AssetOptions(rc.Slug, &rt.Releases[0])
	if len(options) == 0 {
		return labelDim(i18n.T("repo.filter") + ": " + i18n.T("filter.source"))
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
	return combo
}

// repoActions собирает кнопки карточки.
func (u *UI) repoActions(rc config.RepoConfig) fyne.CanvasObject {
	download := widget.NewButton(i18n.T("common.download"), func() {
		go u.mgr.DownloadRepo(context.Background(), rc.Slug, manager.DownloadRequest{Targets: true})
	})
	download.Importance = widget.HighImportance
	releases := widget.NewButton(i18n.T("common.releases"), func() {
		u.st.OpenReleases(rc.Slug)
		u.Rebuild()
	})
	pin := widget.NewButton(i18n.T("common.pin"), func() {
		u.togglePin(rc.Slug)
	})
	folder := widget.NewButton(i18n.T("hero.open_folder"), func() {
		u.openRepoFolder(rc.Slug)
	})
	open := widget.NewButton(i18n.T("common.open"), func() {
		u.openRepoPage(rc.Slug)
	})
	open.Importance = widget.HighImportance
	return container.NewHBox(download, releases, pin, folder, open)
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
	u.pinVersion(slug, rt.LatestStable.Tag)
}

// pinVersion закрепляет или открепляет указанную версию.
func (u *UI) pinVersion(slug, tag string) {
	cfg := u.mgr.Config()
	target := cfg.Repo(slug)
	if target == nil {
		return
	}
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

// removeVersion удаляет скачанную версию (закреплённые удалить нельзя).
func (u *UI) removeVersion(slug, tag string) {
	cfg := u.mgr.Config()
	target := cfg.Repo(slug)
	if target == nil {
		return
	}
	if target.IsPinned(tag) {
		u.st.SetNotice(i18n.TF("repo.version_kept", map[string]string{"{t}": tag}))
		return
	}
	if !u.mgr.RemoveVersion(slug, tag) {
		u.st.SetNotice(i18n.TF("repo.version_kept", map[string]string{"{t}": tag}))
		return
	}
	u.st.SetNotice(i18n.TF("repo.version_deleted", map[string]string{"{t}": tag}))
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

// openRepoPage открывает страницу репозитория в браузере.
func (u *UI) openRepoPage(slug string) {
	url := "https://github.com/" + slug
	if runtime.GOOS == "windows" {
		_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
		return
	}
	_ = exec.Command("open", url).Start()
}

// promptAddRepo просит имя нового репозитория.
func (u *UI) promptAddRepo() {
	entry := widget.NewEntry()
	entry.SetPlaceHolder(i18n.T("repo.slug_hint"))
	showDialog(u.win, i18n.T("repo.add"), entry, func() {
		slug := app.NormalizeSlug(entry.Text)
		if !app.ValidSlug(slug) {
			u.st.SetNotice(i18n.T("repo.add_err"))
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

// promptImport загружает список репозиториев из JSON-файла.
func (u *UI) promptImport() {
	dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		defer reader.Close()
		data, err := os.ReadFile(reader.URI().Path())
		if err != nil {
			u.st.SetNotice(i18n.TF("repo.import_err", map[string]string{"{e}": err.Error()}))
			return
		}
		list, err := config.ParseList(data)
		if err != nil {
			u.st.SetNotice(i18n.TF("repo.import_err", map[string]string{"{e}": err.Error()}))
			return
		}
		cfg := u.mgr.Config()
		n := cfg.ImportList(list)
		_ = u.mgr.SaveConfig(cfg)
		u.st.SetNotice(i18n.TF("repo.import_ok", map[string]string{"{n}": fmt.Sprintf("%d", n)}))
		u.Rebuild()
	}, u.win).Show()
}

// promptExport сохраняет список репозиториев в JSON-файл.
func (u *UI) promptExport() {
	save := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil || writer == nil {
			return
		}
		defer writer.Close()
		path := ""
		if writer.URI() != nil {
			path = writer.URI().Path()
		}
		cfg := u.mgr.Config()
		if err := config.WriteList(path, cfg.ExportList()); err != nil {
			u.st.SetNotice(i18n.TF("repo.export_err", map[string]string{"{e}": err.Error()}))
			return
		}
		u.st.SetNotice(i18n.TF("repo.export_ok", map[string]string{"{p}": path}))
	}, u.win)
	save.SetFileName("repositories.json")
	save.Show()
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
