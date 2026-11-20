// Package gh — клиент GitHub API: список релизов и сведения о репозитории.
package gh

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/KosenkovPro/craftpull-go/internal/i18n"
	"github.com/KosenkovPro/craftpull-go/internal/model"
)

const apiBase = "https://api.github.com"

// UserAgent — строка User-Agent для всех запросов.
const UserAgent = "CraftPull-Go/0.1"

// Client — HTTP-клиент к GitHub.
type Client struct {
	http  *http.Client
	token string
}

// Token возвращает токен, которым клиент авторизуется.
func (c *Client) Token() string { return c.token }

// NewClient собирает клиент. Токен нужен, чтобы поднять лимит запросов API.
func NewClient(token string) (*Client, error) {
	tr := &http.Transport{
		MaxIdleConns:        32,
		MaxIdleConnsPerHost: 8,
		IdleConnTimeout:     90 * time.Second,
		ForceAttemptHTTP2:   true,
	}
	return &Client{
		http: &http.Client{
			Transport: tr,
			Timeout:   60 * time.Second,
		},
		token: strings.TrimSpace(token),
	}, nil
}

func (c *Client) newRequest(ctx context.Context, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Accept", "application/vnd.github+json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	return req, nil
}

// apiError — ошибка обращения к REST API.
type apiError struct {
	message string
	// rateLimited — исчерпан лимит запросов, есть смысл идти запасным путём.
	rateLimited bool
}

func (e *apiError) Error() string { return e.message }

// ErrRateLimited — исчерпан лимит анонимных запросов GitHub API.
var ErrRateLimited = errors.New("github api rate limit reached")

type ghRelease struct {
	TagName     string    `json:"tag_name"`
	Name        *string   `json:"name"`
	Prerelease  bool      `json:"prerelease"`
	Draft       bool      `json:"draft"`
	PublishedAt *string   `json:"published_at"`
	HTMLURL     string    `json:"html_url"`
	Assets      []ghAsset `json:"assets"`
	ZipballURL  string    `json:"zipball_url"`
}

type ghAsset struct {
	Name               string  `json:"name"`
	BrowserDownloadURL string  `json:"browser_download_url"`
	Size               uint64  `json:"size"`
	Digest             *string `json:"digest"`
}

// ListReleases возвращает релизы репозитория, от свежих к старым.
//
// Сначала пробуем REST API. Если лимит запросов исчерпан (у анонимных
// клиентов это 60 запросов в час на IP-адрес), читаем те же данные с
// публичных страниц GitHub — там ограничений нет.
func (c *Client) ListReleases(ctx context.Context, slug string) ([]model.ReleaseInfo, error) {
	releases, err := c.listReleasesAPI(ctx, slug)
	if err == nil {
		return releases, nil
	}
	var ae *apiError
	if !errors.As(err, &ae) || !ae.rateLimited {
		return nil, err
	}
	fallback, ferr := listReleasesWeb(ctx, c.http, slug)
	if ferr != nil {
		return nil, fmt.Errorf("%s (%s)", ferr.Error(), ae.message)
	}
	return fallback, nil
}

func (c *Client) listReleasesAPI(ctx context.Context, slug string) ([]model.ReleaseInfo, error) {
	url := fmt.Sprintf("%s/repos/%s/releases?per_page=40", apiBase, slug)
	req, err := c.newRequest(ctx, url)
	if err != nil {
		return nil, &apiError{message: i18n.TF1("err.network", "{e}", err.Error())}
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, &apiError{message: networkMessage(err, slug)}
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		if resp.Header.Get("x-ratelimit-remaining") == "0" {
			return nil, &apiError{
				message:     i18n.TF1("err.rate_limit", "{slug}", slug),
				rateLimited: true,
			}
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &apiError{message: i18n.TF("err.http_status", map[string]string{
			"{status}": resp.Status,
			"{slug}":   slug,
		})}
	}

	var raw []ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, &apiError{message: i18n.TF1("err.parse", "{e}", err.Error())}
	}

	out := make([]model.ReleaseInfo, 0, len(raw))
	for _, r := range raw {
		if r.Draft {
			continue
		}
		assets := make([]model.AssetInfo, 0, len(r.Assets))
		for _, a := range r.Assets {
			assets = append(assets, model.AssetInfo{
				Name:   a.Name,
				URL:    a.BrowserDownloadURL,
				Size:   a.Size,
				Digest: a.Digest,
			})
		}
		name := r.TagName
		if r.Name != nil && *r.Name != "" {
			name = *r.Name
		}
		out = append(out, model.ReleaseInfo{
			Tag:         r.TagName,
			Name:        name,
			Prerelease:  r.Prerelease,
			Draft:       r.Draft,
			PublishedAt: r.PublishedAt,
			HTMLURL:     r.HTMLURL,
			Assets:      assets,
			SourceZip:   r.ZipballURL,
		})
	}
	return out, nil
}

// RepoInfo — сведения о репозитории.
type RepoInfo struct {
	Description *string
}

type ghRepo struct {
	Description *string `json:"description"`
}

// RepoInfo получает описание репозитория (используется, если описание пустое).
func (c *Client) RepoInfo(ctx context.Context, slug string) (RepoInfo, error) {
	url := fmt.Sprintf("%s/repos/%s", apiBase, slug)
	req, err := c.newRequest(ctx, url)
	if err != nil {
		return RepoInfo{}, errors.New(i18n.TF1("err.network", "{e}", err.Error()))
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return RepoInfo{}, errors.New(i18n.TF1("err.network", "{e}", err.Error()))
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return RepoInfo{}, errors.New(i18n.TF1("err.http_status_short", "{status}", resp.Status))
	}
	var raw ghRepo
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return RepoInfo{}, errors.New(i18n.TF1("err.parse", "{e}", err.Error()))
	}
	if raw.Description != nil && strings.TrimSpace(*raw.Description) == "" {
		raw.Description = nil
	}
	return RepoInfo{Description: raw.Description}, nil
}

// networkMessage переводит сетевую ошибку в понятный текст.
func networkMessage(err error, slug string) string {
	var netErr interface{ Timeout() bool }
	if errors.As(err, &netErr) && netErr.Timeout() {
		return i18n.TF1("err.timeout", "{slug}", slug)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return i18n.TF1("err.timeout", "{slug}", slug)
	}
	msg := err.Error()
	if strings.Contains(msg, "connection refused") || strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "network is unreachable") || strings.Contains(msg, "dial tcp") {
		return i18n.TF1("err.offline", "{slug}", slug)
	}
	return i18n.TF1("err.network", "{e}", err.Error())
}

// readBody читает и закрывает тело ответа.
func readBody(r io.Reader) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
