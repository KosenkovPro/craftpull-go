//go:build cgo && !windows && !darwin

package ui

import (
	"fyne.io/fyne/v2"

	"github.com/KosenkovPro/craftpull-go/internal/manager"
)

// setupTray — заглушка для систем без значка в системном лотке.
func setupTray(a fyne.App, host *windowHost, mgr *manager.Manager, iconRes fyne.Resource) func() {
	return nil
}
