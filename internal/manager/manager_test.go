package manager

import (
	"context"
	"testing"
	"time"

	"github.com/KosenkovPro/craftpull-go/internal/config"
)

// TestRefreshAllNoDeadlock проверяет, что массовая проверка репозиториев
// не зависает.
//
// Раньше список собирался под мьютексом, а внутри вызывался IsStale, который
// берёт тот же мьютекс: получалась взаимоблокировка, и проверки не выполнялись
// никогда.
func TestRefreshAllNoDeadlock(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("CRAFTPULL_CONFIG_DIR", dir)
	t.Setenv("CRAFTPULL_DATA_DIR", dir)

	cfg := config.Config{RootDir: t.TempDir(), MaxParallel: 1, Lang: 0}
	rc := config.NewRepoConfig("octocat/Hello-World")
	rc.Enabled = true
	cfg.Repos = []config.RepoConfig{rc}

	mgr, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer mgr.Close()

	done := make(chan struct{})
	go func() {
		mgr.RefreshAll(context.Background(), false)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(90 * time.Second):
		t.Fatal("RefreshAll не завершилась — взаимоблокировка")
	}

	checks := mgr.Checks()
	if checks.Total() != 1 {
		t.Fatalf("в списке проверок %d пунктов, ожидался 1", checks.Total())
	}
	if checks.Items[0].Status == CheckWaiting {
		t.Fatalf("пункт проверки остался в состоянии ожидания: %+v", checks.Items[0])
	}
}
