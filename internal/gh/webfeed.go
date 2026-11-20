// Запасной источник данных о релизах: публичные страницы GitHub.
//
// REST API GitHub отдаёт анонимным клиентам всего 60 запросов в час на
// IP-адрес. За общим адресом (NAT в офисе, VPN, мобильный интернет) лимит
// заканчивается очень быстро, и обычный список релизов перестаёт работать.
//
// Публичные страницы релизов этому ограничению не подвержены, поэтому при
// отказе API программа читает список релизов и файлы прямо с них:
//   - releases.atom                  — список релизов с тегами, названиями и датами;
//   - releases/latest                — редирект на последний стабильный релиз;
//   - releases/expanded_assets/<tag> — список файлов релиза с размерами
//     и контрольными суммами sha256.
package gh

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/KosenkovPro/craftpull-go/internal/i18n"
	"github.com/KosenkovPro/craftpull-go/internal/model"
)

// maxReleasesWeb — сколько релизов читать из ленты (GitHub отдаёт до 10).
const maxReleasesWeb = 10

type feedEntry struct {
	tag     string
	title   string
	updated string
}

// listReleasesWeb собирает список релизов через публичные страницы GitHub.
func listReleasesWeb(ctx context.Context, client *http.Client, slug string) ([]model.ReleaseInfo, error) {
	feedURL := fmt.Sprintf("https://github.com/%s/releases.atom", slug)
	feed, err := fetchTextRetry(ctx, client, feedURL)
	if err != nil {
		return nil, err
	}
	entries := parseFeed(feed)
	if len(entries) == 0 {
		return nil, fmt.Errorf("%s", i18n.T("err.web_no_releases"))
	}

	// Последний стабильный релиз: страница releases/latest перенаправляет на него.
	stableTag := latestStableTag(ctx, client, slug)

	// Файлы каждого релиза читаем пачками по четыре.
	assets := make([][]model.AssetInfo, len(entries))
	for start := 0; start < len(entries); start += 4 {
		end := start + 4
		if end > len(entries) {
			end = len(entries)
		}
		for i := start; i < end; i++ {
			assets[i] = fetchAssets(ctx, client, slug, entries[i].tag)
		}
	}

	releases := make([]model.ReleaseInfo, 0, len(entries)+1)
	stableSeen := false
	for idx, entry := range entries {
		// Релизы в ленте отсортированы от новых к старым. Всё, что опубликовано
		// позже последнего стабильного, считаем нестабильным.
		isEdge := true
		if stableTag != "" {
			switch {
			case entry.tag == stableTag:
				stableSeen = true
				isEdge = false
			case !stableSeen:
				isEdge = true
			default:
				isEdge = false
			}
		}
		published := entry.updated
		releases = append(releases, model.ReleaseInfo{
			Tag:         entry.tag,
			Name:        entry.title,
			Prerelease:  isEdge,
			Draft:       false,
			PublishedAt: &published,
			HTMLURL:     fmt.Sprintf("https://github.com/%s/releases/tag/%s", slug, entry.tag),
			Assets:      assets[idx],
			SourceZip:   fmt.Sprintf("https://github.com/%s/archive/refs/tags/%s.zip", slug, entry.tag),
		})
	}

	// Стабильный релиз мог не попасть в ленту (например, когда свежие
	// релизы — только pre-release). Добавим его, чтобы канал stable был полон.
	if stableTag != "" {
		found := false
		for _, r := range releases {
			if r.Tag == stableTag {
				found = true
				break
			}
		}
		if !found {
			releases = append(releases, model.ReleaseInfo{
				Tag:         stableTag,
				Name:        stableTag,
				Prerelease:  false,
				Draft:       false,
				PublishedAt: nil,
				HTMLURL:     fmt.Sprintf("https://github.com/%s/releases/tag/%s", slug, stableTag),
				Assets:      fetchAssets(ctx, client, slug, stableTag),
				SourceZip:   fmt.Sprintf("https://github.com/%s/archive/refs/tags/%s.zip", slug, stableTag),
			})
		}
	}
	return releases, nil
}

// fetchText скачивает страницу и возвращает текст.
func fetchText(ctx context.Context, client *http.Client, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", errors.New(i18n.TF1("err.network", "{e}", err.Error()))
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml")
	resp, err := client.Do(req)
	if err != nil {
		return "", errors.New(i18n.TF1("err.network", "{e}", err.Error()))
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", errors.New(i18n.TF1("err.http_status_short", "{status}", resp.Status))
	}
	text, err := readBody(resp.Body)
	if err != nil {
		return "", errors.New(i18n.TF1("err.parse", "{e}", err.Error()))
	}
	return text, nil
}

// fetchTextRetry скачивает страницу с несколькими попытками: прокси и сети
// любят отдавать разовые ошибки, а повтор с новым соединением часто проходит.
func fetchTextRetry(ctx context.Context, client *http.Client, rawURL string) (string, error) {
	last := ""
	for attempt := 0; attempt < 3; attempt++ {
		text, err := fetchText(ctx, client, rawURL)
		if err == nil {
			return text, nil
		}
		last = err.Error()
		if attempt < 2 {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(time.Duration(400*(attempt+1)) * time.Millisecond):
			}
		}
	}
	return "", fmt.Errorf("%s (%s)", last, rawURL)
}

// latestStableTag — тег последнего стабильного релиза (редирект с releases/latest).
func latestStableTag(ctx context.Context, client *http.Client, slug string) string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("https://github.com/%s/releases/latest", slug), nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", UserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ""
	}
	final := resp.Request.URL.String()
	idx := strings.LastIndex(final, "/releases/tag/")
	if idx < 0 {
		return ""
	}
	tag := final[idx+len("/releases/tag/"):]
	if tag == "" || strings.Contains(tag, "/") {
		return ""
	}
	return tag
}

// fetchAssets читает файлы релиза со страницы expanded_assets.
func fetchAssets(ctx context.Context, client *http.Client, slug, tag string) []model.AssetInfo {
	rawURL := fmt.Sprintf("https://github.com/%s/releases/expanded_assets/%s",
		slug, url.PathEscape(tag))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", UserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil
	}
	html, err := readBody(resp.Body)
	if err != nil {
		return nil
	}
	return parseAssets(html, slug)
}

// parseAssets разбирает HTML со списком файлов.
func parseAssets(html, slug string) []model.AssetInfo {
	const marker = "/releases/download/"
	out := make([]model.AssetInfo, 0, 16)
	rest := html
	for {
		pos := strings.Index(rest, marker)
		if pos < 0 {
			break
		}
		after := rest[pos+len(marker):]
		end := strings.Index(after, "\"")
		if end < 0 {
			end = len(after)
		}
		path := after[:end] // {tag}/{file}
		name := ""
		if i := strings.Index(path, "/"); i >= 0 {
			name = percentDecode(path[i+1:])
		}
		// После ссылки идут контрольная сумма и размер — читаем их из окна.
		windowEnd := len(after)
		if i := strings.Index(after[end:], marker); i >= 0 && end+i < windowEnd {
			windowEnd = end + i
		}
		if windowEnd > end+4000 {
			windowEnd = end + 4000
		}
		window := after[end:windowEnd]
		if name != "" {
			out = append(out, model.AssetInfo{
				Name:   name,
				URL:    fmt.Sprintf("https://github.com/%s%s%s", slug, marker, path),
				Size:   findSize(window),
				Digest: findDigest(window),
			})
		}
		rest = after[windowEnd:]
	}
	return out
}

// findDigest — контрольная сумма sha256, если GitHub её указал.
func findDigest(window string) *string {
	pos := strings.Index(window, "sha256:")
	if pos < 0 {
		return nil
	}
	var hex strings.Builder
	for _, c := range window[pos+7:] {
		if hex.Len() == 64 {
			break
		}
		if isHexDigit(c) {
			hex.WriteRune(c)
		}
	}
	if hex.Len() == 64 {
		s := "sha256:" + hex.String()
		return &s
	}
	return nil
}

func isHexDigit(c rune) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

// findSize — размер файла: GitHub отдаёт его текстом рядом с именем.
func findSize(window string) uint64 {
	text := stripTags(window)
	runes := []rune(text)
	i := 0
	for i < len(runes) {
		if runes[i] >= '0' && runes[i] <= '9' {
			start := i
			for i < len(runes) && ((runes[i] >= '0' && runes[i] <= '9') || runes[i] == '.') {
				i++
			}
			number := string(runes[start:i])
			j := i
			for j < len(runes) && runes[j] == ' ' {
				j++
			}
			unitStart := j
			for j < len(runes) && isLetter(runes[j]) {
				j++
			}
			unit := string(runes[unitStart:j])
			if mult, ok := unitMultiplier(unit); ok {
				if value, err := strconv.ParseFloat(number, 64); err == nil {
					return uint64(value * mult)
				}
			}
			if j > i {
				i = j
			}
			continue
		}
		i++
	}
	return 0
}

func isLetter(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// unitMultiplier — во сколько байт равен одной единице GitHub.
func unitMultiplier(unit string) (float64, bool) {
	switch unit {
	case "Bytes", "Byte":
		return 1, true
	case "KB":
		return 1024, true
	case "MB":
		return 1024 * 1024, true
	case "GB":
		return 1024 * 1024 * 1024, true
	case "TB":
		return 1024 * 1024 * 1024 * 1024, true
	}
	return 0, false
}

// stripTags убирает HTML-теги, чтобы остался только текст.
func stripTags(html string) string {
	var b strings.Builder
	b.Grow(len(html))
	depth := 0
	for _, ch := range html {
		switch ch {
		case '<':
			depth++
		case '>':
			if depth > 0 {
				depth--
				b.WriteByte(' ')
			}
		default:
			if depth == 0 {
				b.WriteRune(ch)
			}
		}
	}
	return b.String()
}

// parseFeed разбирает RSS-ленту релизов.
func parseFeed(xml string) []feedEntry {
	out := make([]feedEntry, 0, maxReleasesWeb)
	for _, chunk := range strings.Split(xml, "<entry>")[1:] {
		body := chunk
		if i := strings.Index(body, "</entry>"); i >= 0 {
			body = body[:i]
		}
		// Ссылка на страницу релиза: в ней же тег.
		link := ""
		if i := strings.Index(body, "<link rel=\"alternate\""); i >= 0 {
			rest := body[i+len("<link rel=\"alternate\""):]
			if j := strings.Index(rest, "href=\""); j >= 0 {
				href := rest[j+len("href=\""):]
				if k := strings.Index(href, "\""); k >= 0 {
					link = href[:k]
				}
			}
		}
		idx := strings.LastIndex(link, "/releases/tag/")
		if idx < 0 {
			continue
		}
		tag := percentDecode(link[idx+len("/releases/tag/"):])
		if tag == "" {
			continue
		}
		title := tag
		if t := between(body, "<title>", "</title>"); t != "" {
			title = htmlUnescape(t)
		}
		updated := strings.TrimSpace(between(body, "<updated>", "</updated>"))
		out = append(out, feedEntry{tag: tag, title: title, updated: updated})
		if len(out) >= maxReleasesWeb {
			break
		}
	}
	return out
}

// between возвращает содержимое между двумя маркерами.
func between(s, open, close string) string {
	i := strings.Index(s, open)
	if i < 0 {
		return ""
	}
	rest := s[i+len(open):]
	j := strings.Index(rest, close)
	if j < 0 {
		return ""
	}
	return rest[:j]
}

// htmlUnescape раскрывает HTML-сущности.
func htmlUnescape(s string) string {
	r := strings.NewReplacer(
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", "\"",
		"&#39;", "'",
		"&#x27;", "'",
		"&apos;", "'",
		"&amp;", "&",
	)
	return r.Replace(s)
}

// percentDecode раскрывает percent-encoding в имени файла или теге.
func percentDecode(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) {
			if v, err := strconv.ParseUint(s[i+1:i+3], 16, 8); err == nil {
				b.WriteByte(byte(v))
				i += 2
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
