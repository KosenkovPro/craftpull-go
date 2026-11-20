//go:build cgo

// Панель релизов: открывается справа внутри страницы, выбор файлов галочками.
package ui

import (
	"context"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/KosenkovPro/craftpull-go/internal/i18n"
	"github.com/KosenkovPro/craftpull-go/internal/manager"
	"github.com/KosenkovPro/craftpull-go/internal/model"
)

// buildReleases собирает панель релизов репозитория.
func (u *UI) buildReleases(slug string) fyne.CanvasObject {
	releases := u.mgr.Releases(slug)
	if len(releases) == 0 {
		releases = u.loadReleases(slug)
	}

	header := container.NewBorder(nil, nil,
		labelText(i18n.TF("popup.title", map[string]string{"{s}": slug}), true),
		widget.NewButton("×", func() {
			u.st.CloseReleases()
			u.Rebuild()
		}),
	)

	hint := widget.NewLabel(i18n.T("releases.hint"))
	hint.TextStyle = fyne.TextStyle{}
	// Первый релиз раскрыт сразу, а файл под текущую ОС отмечен автоматически.
	if u.st.Expanded() == "" && len(releases) > 0 {
		u.st.SetExpanded(releases[0].Tag)
	}
	for i := range releases {
		if releases[i].IsEdge() {
			continue
		}
		u.st.Preselect(slug, &releases[i])
		break
	}

	list := container.NewVBox()
	for i := range releases {
		list.Add(u.releaseCard(slug, &releases[i]))
	}

	// Липкая нижняя панель: итог и кнопки — всегда внизу, не прокручиваются.
	summary := widget.NewLabel(u.st.SummaryText(releases))
	summary.TextStyle = fyne.TextStyle{Bold: true}

	count, size, _ := u.st.SelectionSummary(releases)
	download := widget.NewButtonWithIcon(i18n.T("releases.download_selected"), nil, func() {
		assets := u.st.CollectSelection(releases)
		if len(assets) == 0 {
			return
		}
		tag := ""
		tags := u.st.SelectedTags(releases)
		if len(tags) > 0 {
			tag = tags[0]
		}
		go u.mgr.DownloadRepo(context.Background(), slug, manager.DownloadRequest{
			Tag:    tag,
			Assets: assets,
		})
	})
	if count == 0 {
		download.Disable()
	}
	clear := widget.NewButton(i18n.T("releases.clear"), func() {
		u.st.ClearSelection()
		u.Rebuild()
	})
	bottom := container.NewVBox(
		summary,
		container.NewBorder(nil, nil, nil,
			container.NewHBox(download, clear),
			widget.NewLabel(fmt.Sprintf("%s · %s", size, i18n.T("popup.size"))),
		),
	)

	// Border, а не VBox: список релизов должен занимать всю высоту панели,
	// а липкая нижняя панель — оставаться на месте.
	top := container.NewVBox(header, hint)
	panel := container.NewBorder(top, bottom, nil, nil, container.NewScroll(list))
	return panel
}

// loadReleases подгружает релизы репозитория, если их ещё нет в памяти.
func (u *UI) loadReleases(slug string) []model.ReleaseInfo {
	go func() {
		_, _ = u.mgr.RefreshRepo(context.Background(), slug)
	}()
	return u.mgr.Releases(slug)
}

// releaseCard собирает карточку одного релиза.
func (u *UI) releaseCard(slug string, rel *model.ReleaseInfo) fyne.CanvasObject {
	expanded := u.st.Expanded() == rel.Tag

	toggle := widget.NewButton("v", func() {
		u.st.ToggleExpand(rel.Tag)
		u.Rebuild()
	})
	if !expanded {
		toggle.SetText(">")
	}

	head := container.NewHBox(
		toggle,
		labelText(rel.Tag, true),
		chip(rel.Channel().Label(), channelColor(rel)),
	)
	if rel.PublishedAt != nil {
		head.Add(widget.NewLabel(model.HumanDate(*rel.PublishedAt)))
	}
	head.Add(layout.NewSpacer())

	card := container.NewVBox(head)
	if expanded {
		card.Add(u.releaseFiles(slug, rel))
	}
	return card
}

// releaseFiles собирает список файлов релиза с галочками.
func (u *UI) releaseFiles(slug string, rel *model.ReleaseInfo) fyne.CanvasObject {
	options := manager.AssetOptions(slug, rel)
	box := container.NewVBox()

	for _, opt := range options {
		box.Add(u.fileRow(slug, rel, opt.Asset.Name, opt.Kind.Label()))
	}
	// Остальные файлы релиза — под раскрытием.
	extras := make([]model.AssetInfo, 0, len(rel.Assets))
	for _, a := range rel.Assets {
		if manager.IsNoiseAsset(a.Name) {
			continue
		}
		known := false
		for _, opt := range options {
			if opt.Asset.Name == a.Name {
				known = true
				break
			}
		}
		if !known {
			extras = append(extras, a)
		}
	}
	if len(extras) > 0 {
		more := widget.NewButton(i18n.TF("releases.more_files", map[string]string{
			"{n}": fmt.Sprintf("%d", len(extras)),
		}), func() {
			u.Rebuild()
		})
		box.Add(more)
		for _, a := range extras {
			box.Add(u.fileRow(slug, rel, a.Name, i18n.T("releases.other")))
		}
	}

	// Архив исходников — отдельным пунктом.
	source := model.SourceZip(rel.Tag, rel.SourceZip)
	box.Add(u.fileRow(slug, rel, source.Name, i18n.T("releases.source")))

	return box
}

// fileRow собирает строку файла с галочкой.
func (u *UI) fileRow(slug string, rel *model.ReleaseInfo, name, kindLabel string) fyne.CanvasObject {
	checked := u.st.IsSelected(rel.Tag, name)
	// SetChecked тоже вызывает обработчик, поэтому на время установки
	// значения он молчит — иначе отметка тут же снималась бы.
	applying := true
	check := widget.NewCheck("", func(on bool) {
		if applying {
			return
		}
		u.st.ToggleAsset(rel.Tag, name)
		u.Rebuild()
	})
	check.SetChecked(checked)
	applying = false

	size := ""
	for _, a := range rel.Assets {
		if a.Name == name {
			size = model.HumanSize(a.Size)
			break
		}
	}

	row := container.NewBorder(nil, nil,
		container.NewHBox(check, widget.NewLabel(kindLabel)),
		widget.NewLabel(size),
		widget.NewLabel(name),
	)
	return row
}

// channelColor возвращает цвет канала релиза.
func channelColor(rel *model.ReleaseInfo) interface{ RGBA() (r, g, b, a uint32) } {
	if rel.IsEdge() {
		return colEdge
	}
	return colOK
}
