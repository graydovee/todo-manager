// Package i18n provides a minimal, dependency-free translation facility for the
// desktop client. It supports the {{name}} interpolation style used by the web
// frontend and is safe for concurrent use (the language can change at runtime).
package i18n

import (
	"fmt"
	"strings"
	"sync"
)

// Lang is a language code ("en" or "zh").
type Lang string

const (
	// En is English.
	En Lang = "en"
	// Zh is Simplified Chinese.
	Zh Lang = "zh"
)

// Translator looks up translated strings for the current language.
type Translator struct {
	mu   sync.RWMutex
	lang Lang
	dict map[string]string
}

// Default is the application-wide translator. It is safe to call concurrently.
var Default = &Translator{lang: En, dict: en}

// T translates key for the current language, applying {{name}} interpolation
// with the alternating name/value pairs in args (e.g. T("x", "count", 3)).
// Missing keys fall back to English, then to the key itself.
func (t *Translator) T(key string, args ...any) string {
	t.mu.RLock()
	lang := t.lang
	dict := t.dict
	t.mu.RUnlock()

	val, ok := dict[key]
	if !ok {
		// Fallback to English.
		val, ok = en[key]
		if !ok {
			val = key
		}
	}
	return interpolate(val, args, lang)
}

// SetLang switches the active language and reloads its dictionary.
func (t *Translator) SetLang(l Lang) {
	dict := en
	if l == Zh {
		dict = zh
	}
	t.mu.Lock()
	t.lang = l
	t.dict = dict
	t.mu.Unlock()
}

// Lang returns the current language.
func (t *Translator) Lang() Lang {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.lang
}

// T is shorthand for Default.T.
func T(key string, args ...any) string { return Default.T(key, args...) }

// interpolate replaces {{name}} placeholders. args is an alternating list of
// string key / any value pairs. Unknown placeholders are left intact.
func interpolate(val string, args []any, _ Lang) string {
	if len(args) == 0 {
		return val
	}
	for i := 0; i+1 < len(args); i += 2 {
		name, ok := args[i].(string)
		if !ok {
			continue
		}
		val = strings.ReplaceAll(val, "{{"+name+"}}", fmt.Sprint(args[i+1]))
	}
	return val
}
