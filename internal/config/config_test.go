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

// TestDefaultCatalog проверяет каталог программ: у каждого репозитория из
// списка по умолчанию есть назначение и категория, а сам список не содержит
// повторов.
func TestDefaultCatalog(t *testing.T) {
	seen := make(map[string]bool, len(DefaultRepos))
	for _, slug := range DefaultRepos {
		if seen[slug] {
			t.Fatalf("репозиторий %s повторяется в списке по умолчанию", slug)
		}
		seen[slug] = true

		if purpose, ok := DefaultPurposes[slug]; !ok || purpose == "" {
			t.Fatalf("у репозитория %s нет назначения в таблице", slug)
		}
		if cat, ok := DefaultCategories[slug]; !ok || cat == "" {
			t.Fatalf("у репозитория %s нет категории в таблице", slug)
		}
	}

	// Программы каталога, которые проверялись на наличие релизов.
	want := []string{
		"obsproject/obs-studio",
		"NickeManarin/ScreenToGif",
		"mifi/lossless-cut",
		"HandBrake/HandBrake",
		"audacity/audacity",
		"mpv-player/mpv",
		"jeffvli/feishin",
		"SV-stark/FileConverter-rs",
		"d2phap/ImageGlass",
		"darktable-org/darktable",
		"FreeCAD/FreeCAD",
		"microsoft/PowerToys",
		"dundee/gdu",
		"iterate-ch/cyberduck",
		"qbittorrent/qBittorrent",
		"pbatard/rufus",
		"rustdesk/rustdesk",
		"keepassxreboot/keepassxc",
		"organicmaps/organicmaps",
		"kiwix/kiwix-desktop",
		"kiwix/kiwix-android",
		"passbolt/passbolt_api",
		"immich-app/immich",
		"owncloud/ocis",
		"nextcloud/server",
		"Freika/dawarich",
		"jellyfin/jellyfin",
		"kieraneglin/pinchflat",
		"navidrome/navidrome",
		"advplyr/audiobookshelf",
		"Kareadita/Kavita",
		"owncast/owncast",
		"kiwix/kiwix-tools",
		"VERT-sh/VERT",
		"VERT-sh/vertd",
	}
	for _, slug := range want {
		if !seen[slug] {
			t.Fatalf("репозиторий %s не добавлен в список по умолчанию", slug)
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
	if rc.Category != DefaultCategories["storytold/pdfcraft"] {
		t.Fatalf("категория не подставлена: %q", rc.Category)
	}
	if cfg.Repo("flameshot-org/flameshot") == nil {
		t.Fatal("новый репозиторий по умолчанию не добавился")
	}
}

// TestCategoryRoundTrip проверяет, что категория программы переживает
// экспорт и импорт списка репозиториев.
func TestCategoryRoundTrip(t *testing.T) {
	cfg := Config{Repos: []RepoConfig{
		{Slug: "owner/app", Enabled: true, Purpose: "Работа с фото", Category: CatGraphics},
	}}

	data, err := json.Marshal(cfg.ExportList())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"category"`) {
		t.Fatalf("в файле списка нет поля category: %s", data)
	}

	var file RepoListFile
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	other := Config{}
	if other.ImportList(&file) != 1 {
		t.Fatal("репозиторий не добавился при импорте")
	}
	if got := other.Repos[0].Category; got != CatGraphics {
		t.Fatalf("категория после импорта: %q", got)
	}
}
