// CraftPull — менеджер релизов и исходников для репозиториев GitHub.
//
// Go-версия: та же логика, что и в Rust-реализации, но собирается под
// Windows, macOS (Intel и Apple Silicon) и Linux.
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/KosenkovPro/craftpull-go/internal/app"
	"github.com/KosenkovPro/craftpull-go/internal/buildinfo"
	"github.com/KosenkovPro/craftpull-go/internal/config"
	"github.com/KosenkovPro/craftpull-go/internal/console"
	"github.com/KosenkovPro/craftpull-go/internal/i18n"
	"github.com/KosenkovPro/craftpull-go/internal/icon"
	"github.com/KosenkovPro/craftpull-go/internal/icons"
	"github.com/KosenkovPro/craftpull-go/internal/manager"
	"github.com/KosenkovPro/craftpull-go/internal/model"
	"github.com/KosenkovPro/craftpull-go/internal/shortcut"
	"github.com/KosenkovPro/craftpull-go/internal/ui"
)

// Version — версия программы.
const Version = buildinfo.Version

func main() {
	// Оконная сборка (-H windowsgui) не имеет консоли: присоединяем
	// терминал родителя, чтобы --help и --version печатались как обычно.
	console.Attach()

	args := os.Args[1:]

	if has(args, "--help", "-h") {
		printHelp()
		return
	}
	if has(args, "--version", "-V") {
		fmt.Printf("CraftPull %s\n", Version)
		return
	}
	if has(args, "--install-shortcut") {
		path, err := shortcut.Install()
		if err != nil {
			fmt.Fprintf(os.Stderr, "CraftPull: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Shortcut created: %s\n", path)
		return
	}
	// Экспорт иконки в PNG (для сборок и оформления).
	if target, ok := argValue(args, "--export-icon"); ok {
		if err := os.WriteFile(target, icon.PNG(256), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "CraftPull: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Icon saved: %s\n", target)
		return
	}
	// Иконки сторонних приложений отключаются флагом: ни в репозитории,
	// ни на скриншотах документации не должно быть чужих логотипов.
	if has(args, "--no-icons") {
		icons.SetDisabled(true)
	}

	cfg := config.Load()
	i18n.Set(cfg.Lang)

	mgr, err := manager.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "CraftPull: %v\n", err)
		os.Exit(1)
	}
	defer mgr.Close()

	mgr.Log(model.LogInfo, i18n.TF("log.started", map[string]string{"{dir}": cfg.RootDir}))

	// Сквозная проверка: опросить GitHub, скачать целевые релизы, показать результат.
	if slug, ok := argValue(args, "--selftest"); ok {
		runSelfTest(mgr, slug)
		return
	}

	opts := ui.Options{
		View:         cfg.ViewMode,
		AutoRefresh:  !has(args, "--no-refresh"),
		ForceRefresh: has(args, "--refresh"),
		Hidden:       has(args, "--tray") || cfg.StartMinimized,
	}
	if v, ok := argValue(args, "--view"); ok {
		switch config.ViewMode(v) {
		case config.ViewGrid, config.ViewList, config.ViewLight:
			opts.View = config.ViewMode(v)
		}
	}
	if t, ok := argValue(args, "--tab"); ok {
		switch strings.ToLower(t) {
		case "settings":
			opts.Tab = app.TabSettings
		case "logs":
			opts.Tab = app.TabLogs
		default:
			opts.Tab = app.TabRepos
		}
	}
	if slug, ok := argValue(args, "--popup"); ok {
		opts.Popup = slug
	}
	if slug, ok := argValue(args, "--hero"); ok {
		opts.Hero = slug
	}
	if slug, ok := argValue(args, "--download"); ok {
		opts.Download = slug
	}

	st := app.New(opts.View, cfg.Lang)
	st.SetTab(opts.Tab)

	if err := ui.Run(mgr, st, opts); err != nil {
		fmt.Fprintf(os.Stderr, "CraftPull: %v\n", err)
		os.Exit(1)
	}
	mgr.Log(model.LogInfo, i18n.T("log.exit"))
}

// runSelfTest — сквозная проверка без окна.
func runSelfTest(mgr *manager.Manager, slug string) {
	ctx := context.Background()
	if slug == "" {
		fmt.Println("CraftPull: self-test without a repository")
		return
	}
	count, err := mgr.RefreshRepo(ctx, slug)
	if err != nil {
		fmt.Fprintf(os.Stderr, "CraftPull: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("%s: %d releases\n", slug, count)
	go mgr.DownloadRepo(ctx, slug, manager.DownloadRequest{Targets: true})
	mgr.WaitIdle(ctx)
	for _, entry := range mgr.Logs() {
		fmt.Printf("%s %s\n", entry.Time, entry.Text)
	}
}

// has проверяет наличие флага.
func has(args []string, names ...string) bool {
	for _, a := range args {
		for _, n := range names {
			if a == n {
				return true
			}
		}
	}
	return false
}

// argValue возвращает значение флага.
func argValue(args []string, key string) (string, bool) {
	for i, a := range args {
		if a == key && i+1 < len(args) {
			return args[i+1], true
		}
	}
	return "", false
}

func printHelp() {
	fmt.Print(`CraftPull — release & source manager for GitHub

Usage:
  craftpull                      open the window
  craftpull --tray               start minimized in the tray
  craftpull --install-shortcut   install the program and create a shortcut
  craftpull --export-icon <png>  save the application icon as PNG
  craftpull --selftest [slug]    end-to-end check: query GitHub and download a release
  craftpull --tab <repos|settings|logs>   open a specific tab
  craftpull --view <grid|list|light>      open a specific view mode
  craftpull --popup <owner/repo>          open the releases panel of a repository
  craftpull --hero <owner/repo>           open the grid with the card unfolded
  craftpull --download <owner/repo>       start the download of a repository
  craftpull --no-icons          do not download or show third-party icons
  craftpull --no-refresh        do not contact GitHub on start, use the cache
  craftpull --refresh           refresh the repositories even if the cache is fresh
  craftpull --version           show the version
`)
}
