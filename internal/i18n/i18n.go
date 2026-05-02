package i18n

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"
	"sync"
)

type Locale struct {
	Code     string
	Messages map[string]string
}

type I18n struct {
	mu       sync.RWMutex
	locales  map[string]*Locale
	fallback string
}

func New(fallback string) (*I18n, error) {
	if fallback == "" {
		fallback = "tr"
	}
	return &I18n{
		locales:  make(map[string]*Locale),
		fallback: fallback,
	}, nil
}

func (i *I18n) LoadFromFS(fsys fs.FS, dir string) error {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return fmt.Errorf("read locale dir %q: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		code := strings.TrimSuffix(entry.Name(), ".json")
		path := dir + "/" + entry.Name()

		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return fmt.Errorf("read locale file %q: %w", path, err)
		}

		messages := make(map[string]string)
		if err := json.Unmarshal(data, &messages); err != nil {
			return fmt.Errorf("parse locale file %q: %w", path, err)
		}

		i.mu.Lock()
		i.locales[code] = &Locale{
			Code:     code,
			Messages: messages,
		}
		i.mu.Unlock()
	}

	return nil
}

func (i *I18n) T(lang, key string) string {
	i.mu.RLock()
	defer i.mu.RUnlock()

	if loc, ok := i.locales[lang]; ok {
		if msg, ok := loc.Messages[key]; ok {
			return msg
		}
	}

	if lang != i.fallback {
		if loc, ok := i.locales[i.fallback]; ok {
			if msg, ok := loc.Messages[key]; ok {
				return msg
			}
		}
	}

	return key
}

func (i *I18n) WithParams(lang, key string, params map[string]string) string {
	msg := i.T(lang, key)
	for k, v := range params {
		msg = strings.ReplaceAll(msg, "{{"+k+"}}", v)
	}
	return msg
}

func (i *I18n) Fallback() string {
	return i.fallback
}

func (i *I18n) HasLocale(code string) bool {
	i.mu.RLock()
	defer i.mu.RUnlock()
	_, ok := i.locales[code]
	return ok
}

func (i *I18n) AvailableLocales() []string {
	i.mu.RLock()
	defer i.mu.RUnlock()
	codes := make([]string, 0, len(i.locales))
	for code := range i.locales {
		codes = append(codes, code)
	}
	return codes
}
