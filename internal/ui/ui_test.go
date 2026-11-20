//go:build cgo

package ui

// Мелкие проверки служебных подписей интерфейса: они показываются в
// карточках репозиториев и легко ломаются при правке переводов.

import (
	"strings"
	"testing"
	"time"

	"github.com/KosenkovPro/craftpull-go/internal/config"
	"github.com/KosenkovPro/craftpull-go/internal/i18n"
	"github.com/KosenkovPro/craftpull-go/internal/manager"
	"github.com/KosenkovPro/craftpull-go/internal/model"
)

// TestCheckedLabel проверяет, что подпись времени проверки меняется
// вместе с возрастом проверки и не зависит от языка интерфейса.
func TestCheckedLabel(t *testing.T) {
	i18n.Set(i18n.EN)
	defer i18n.Set(i18n.EN)

	now := time.Now().Format(time.RFC3339)
	if got := checkedLabel(now); !strings.Contains(got, "just now") {
		t.Fatalf("подпись свежей проверки: %q", got)
	}

	old := time.Now().Add(-3 * time.Hour).Format(time.RFC3339)
	if got := checkedLabel(old); !strings.Contains(got, "h ago") {
		t.Fatalf("подпись старой проверки: %q", got)
	}

	// Неразбираемое время не должно ронять интерфейс.
	if got := checkedLabel("не дата"); got == "" {
		t.Fatal("подпись для неразбираемой даты пуста")
	}
}

// TestCountEnabled проверяет подсчёт включённых репозиториев.
func TestCountEnabled(t *testing.T) {
	cfg := config.Config{Repos: []config.RepoConfig{
		{Slug: "a/b", Enabled: true},
		{Slug: "c/d", Enabled: false},
		{Slug: "e/f", Enabled: true},
	}}
	if got := countEnabled(cfg); got != 2 {
		t.Fatalf("включённых репозиториев: %d, ожидалось 2", got)
	}
}

// TestVersionLine проверяет, что строка версии содержит тег релиза и канал.
func TestVersionLine(t *testing.T) {
	rt := manager.RepoRuntime{LatestStable: &model.ReleaseInfo{Tag: "v1.2.3"}}
	if got := versionLine(rt); !strings.Contains(got, "v1.2.3") {
		t.Fatalf("строка версии: %q", got)
	}
	empty := versionLine(manager.RepoRuntime{})
	if empty == "" {
		t.Fatal("строка версии без релизов пуста")
	}
}
