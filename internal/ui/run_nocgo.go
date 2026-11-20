//go:build !cgo

package ui

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/KosenkovPro/craftpull-go/internal/app"
	"github.com/KosenkovPro/craftpull-go/internal/buildinfo"
	"github.com/KosenkovPro/craftpull-go/internal/i18n"
	"github.com/KosenkovPro/craftpull-go/internal/manager"
	"github.com/KosenkovPro/craftpull-go/internal/model"
)

// Run запускает консольный вариант программы.
//
// Графическое окно требует cgo (драйвер OpenGL), поэтому там, где cgo
// недоступен, программа работает в терминале: список репозиториев,
// проверка, загрузка и журнал.
func Run(mgr *manager.Manager, st *app.State, opts Options) error {
	ctx := context.Background()

	if opts.AutoRefresh {
		go mgr.RefreshAll(ctx, opts.ForceRefresh)
	}
	if opts.Download != "" {
		go mgr.DownloadRepo(ctx, opts.Download, manager.DownloadRequest{Targets: true})
	}

	printHeader(mgr)
	deadline := time.Now().Add(3 * time.Second)
	for mgr.IsBusy() && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}
	printRepos(mgr)
	printTasks(mgr)
	return nil
}

func printHeader(mgr *manager.Manager) {
	fmt.Printf("%s — %s\n", buildinfo.Name, i18n.T("app.subtitle"))
	cfg := mgr.Config()
	fmt.Printf("%s: %s   %s: %d   %s: %d\n\n",
		i18n.T("settings.folder"), cfg.RootDir,
		i18n.T("settings.parallel"), cfg.MaxParallel,
		i18n.T("status.total"), len(cfg.Repos))
}

func printRepos(mgr *manager.Manager) {
	cfg := mgr.Config()
	byAuthor := make(map[string][]string)
	for _, r := range cfg.Repos {
		byAuthor[r.Author()] = append(byAuthor[r.Author()], r.Slug)
	}
	authors := make([]string, 0, len(byAuthor))
	for a := range byAuthor {
		authors = append(authors, a)
	}
	sort.Strings(authors)

	for _, author := range authors {
		fmt.Printf("%s %s\n", i18n.T("repo.group"), author)
		for _, slug := range byAuthor[author] {
			rt, ok := mgr.RepoRuntime(slug)
			if !ok {
				continue
			}
			version := "—"
			if rt.LatestStable != nil {
				version = rt.LatestStable.Tag
			} else if rt.LatestEdge != nil {
				version = rt.LatestEdge.Tag
			}
			mark := " "
			if rc := cfg.Repo(slug); rc != nil && rc.Enabled {
				mark = "*"
			}
			fmt.Printf("  %s %-40s %-10s %s\n", mark, slug, version, i18n.T("repo.checked"))
		}
		fmt.Println()
	}
}

func printTasks(mgr *manager.Manager) {
	tasks := mgr.Tasks().All()
	if len(tasks) == 0 {
		return
	}
	fmt.Println(i18n.T("downloads.title"))
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].Seq < tasks[j].Seq })
	for _, t := range tasks {
		fmt.Printf("  #%d %s  %s  %.0f%%  %s  %s\n",
			t.Seq, t.Asset, t.State.Label(), t.Fraction()*100,
			model.HumanSpeed(t.Speed), model.HumanSize(t.Downloaded))
	}
	fmt.Println()
}
