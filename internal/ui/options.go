// Общие для обоих режимов (окно и терминал) параметры интерфейса.
package ui

import (
	"github.com/KosenkovPro/craftpull-go/internal/app"
	"github.com/KosenkovPro/craftpull-go/internal/buildinfo"
	"github.com/KosenkovPro/craftpull-go/internal/config"
)

// Version — версия программы.
const Version = buildinfo.Version

// Options — параметры запуска интерфейса.
type Options struct {
	View         config.ViewMode
	Tab          app.Tab
	Popup        string
	Hero         string
	Download     string
	AutoRefresh  bool
	ForceRefresh bool
	// Hidden — запустить в трее, не показывая окно.
	Hidden bool
}
