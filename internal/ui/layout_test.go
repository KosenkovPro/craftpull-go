//go:build cgo

package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/KosenkovPro/craftpull-go/internal/app"
	"github.com/KosenkovPro/craftpull-go/internal/config"
	"github.com/KosenkovPro/craftpull-go/internal/manager"
)

// TestLayout проверяет, что окно заполняется целиком, а карточки репозиториев
// имеют осмысленный размер.
//
// Проверка ловит типичную ошибку раскладки Fyne: вертикальная коробка
// отдаёт элементам только минимальную высоту, и рабочая область схлопывается.
func TestLayout(t *testing.T) {
	// Настройки и журнал — во временную папку: тест не должен трогать
	// файлы пользователя.
	dir := t.TempDir()
	t.Setenv("CRAFTPULL_CONFIG_DIR", dir)
	t.Setenv("CRAFTPULL_DATA_DIR", dir)

	test.NewApp()

	cfg := config.Config{RootDir: t.TempDir(), MaxParallel: 4, Lang: 1}
	cfg.Repos = []config.RepoConfig{
		config.NewRepoConfig("storytold/photocraft"),
		config.NewRepoConfig("storytold/gridcraft"),
	}
	mgr, err := manager.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Close()

	st := app.New(config.ViewList, 1)
	u := NewUI(mgr, st)
	win := test.NewWindow(nil)
	win.Resize(fyne.NewSize(1296, 879))
	win.SetContent(u.Build(win))
	win.Show()

	if got := win.Content().Size().Height; got < 800 {
		t.Fatalf("высота окна %v — рабочая область не растянулась", win.Content().Size())
	}
	if got := u.body.Size().Height; got < 600 {
		t.Fatalf("высота рабочей области %v — список репозиториев не виден", got)
	}

	card := u.repoCard(cfg.Repos[0], false)
	if card.MinSize().Height < 80 {
		t.Fatalf("высота карточки %v — карточка схлопнулась", card.MinSize())
	}
}
