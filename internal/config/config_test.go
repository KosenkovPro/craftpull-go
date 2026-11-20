package config

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestDefaultRepos проверяет, что в списке по умолчанию есть Flameshot
// и у каждого репозитория задано назначение программы.
func TestDefaultRepos(t *testing.T) {
	found := false
	for _, slug := range DefaultRepos {
		if slug == "flameshot-org/flameshot" {
			found = true
		}
	}
	if !found {
		t.Fatal("flameshot-org/flameshot не добавлен в список по умолчанию")
	}

	cfg := Default()
	for _, rc := range cfg.Repos {
		if rc.Purpose == "" {
			t.Fatalf("у репозитория %s нет назначения", rc.Slug)
		}
	}
}

// TestPurposeRoundTrip проверяет, что назначение программы переживает
// экспорт и импорт списка репозиториев.
func TestPurposeRoundTrip(t *testing.T) {
	cfg := Config{Repos: []RepoConfig{
		{Slug: "owner/app", Enabled: true, Purpose: "Работа с фото"},
	}}

	data, err := json.Marshal(cfg.ExportList())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"purpose"`) {
		t.Fatalf("в файле списка нет поля purpose: %s", data)
	}

	var file RepoListFile
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	other := Config{}
	if other.ImportList(&file) != 1 {
		t.Fatal("репозиторий не добавился при импорте")
	}
	if got := other.Repos[0].Purpose; got != "Работа с фото" {
		t.Fatalf("назначение после импорта: %q", got)
	}
}

// TestEnsureReposFillsPurpose проверяет, что старый конфиг без назначений
// получает их при загрузке.
func TestEnsureReposFillsPurpose(t *testing.T) {
	cfg := Config{Repos: []RepoConfig{{Slug: "storytold/pdfcraft"}}}
	cfg.ensureRepos()

	rc := cfg.Repo("storytold/pdfcraft")
	if rc == nil {
		t.Fatal("репозиторий пропал")
	}
	if rc.Purpose != DefaultPurposes["storytold/pdfcraft"] {
		t.Fatalf("назначение не подставлено: %q", rc.Purpose)
	}
	if cfg.Repo("flameshot-org/flameshot") == nil {
		t.Fatal("новый репозиторий по умолчанию не добавился")
	}
}
