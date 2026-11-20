package i18n

import (
	"strings"
	"testing"
)

// TestTable проверяет таблицу переводов: у ключей должны быть и английский,
// и русский вариант, а подстановки {…} — одинаковые в обоих.
func TestTable(t *testing.T) {
	seen := make(map[string]bool, len(table()))
	for key, e := range table() {
		if seen[key] {
			t.Errorf("дубликат ключа %q", key)
		}
		seen[key] = true

		if strings.TrimSpace(e.en) == "" {
			t.Errorf("ключ %q: пустой английский текст", key)
		}
		if strings.TrimSpace(e.ru) == "" {
			t.Errorf("ключ %q: пустой русский текст", key)
		}
		for _, placeholder := range placeholders(e.en) {
			if !strings.Contains(e.ru, placeholder) {
				t.Errorf("ключ %q: в русском варианте нет %s", key, placeholder)
			}
		}
		for _, placeholder := range placeholders(e.ru) {
			if !strings.Contains(e.en, placeholder) {
				t.Errorf("ключ %q: в английском варианте нет %s", key, placeholder)
			}
		}
	}
	if len(seen) < 100 {
		t.Errorf("в таблице всего %d ключей — переводы потеряны", len(seen))
	}
}

// placeholders возвращает подстановки вида {n} из строки.
func placeholders(text string) []string {
	var out []string
	for {
		start := strings.Index(text, "{")
		if start < 0 {
			return out
		}
		end := strings.Index(text[start:], "}")
		if end < 0 {
			return out
		}
		out = append(out, text[start:start+end+1])
		text = text[start+end+1:]
	}
}
