// Package cache — кэш релизов на диске: переживает перезапуск и бережёт лимит GitHub API.
package cache

import (
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/KosenkovPro/craftpull-go/internal/model"
)

// Entry — сохранённые релизы одного репозитория.
type Entry struct {
	Slug      string              `json:"slug"`
	UpdatedAt time.Time           `json:"updated_at"`
	Releases  []model.ReleaseInfo `json:"releases"`
}

// Cache — кэш релизов в памяти и на диске.
type Cache struct {
	mu      sync.RWMutex
	path    string
	entries map[string]Entry
}

// New загружает кэш с диска.
func New(path string) *Cache {
	c := &Cache{path: path, entries: make(map[string]Entry)}
	data, err := os.ReadFile(path)
	if err != nil {
		return c
	}
	var entries []Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return c
	}
	for _, e := range entries {
		c.entries[e.Slug] = e
	}
	return c
}

// Get возвращает релизы репозитория из кэша.
func (c *Cache) Get(slug string) ([]model.ReleaseInfo, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.entries[slug]
	if !ok || len(e.Releases) == 0 {
		return nil, false
	}
	return e.Releases, true
}

// UpdatedAt возвращает время последнего обновления репозитория.
func (c *Cache) UpdatedAt(slug string) (time.Time, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.entries[slug]
	if !ok {
		return time.Time{}, false
	}
	return e.UpdatedAt, true
}

// Put записывает релизы репозитория в кэш.
func (c *Cache) Put(slug string, releases []model.ReleaseInfo) {
	c.mu.Lock()
	c.entries[slug] = Entry{Slug: slug, UpdatedAt: time.Now(), Releases: releases}
	c.mu.Unlock()
	_ = c.Save()
}

// Save записывает кэш на диск.
func (c *Cache) Save() error {
	c.mu.RLock()
	entries := make([]Entry, 0, len(c.entries))
	for _, e := range c.entries {
		entries = append(entries, e)
	}
	c.mu.RUnlock()

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dirOf(c.path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(c.path, data, 0o644)
}

func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[:i]
		}
	}
	return "."
}
