package i18n

import "strings"

// DetectLang returns the user's preferred language. On Windows it reads the
// user default locale; elsewhere it inspects the LANG environment variable. The
// result is Zh when the locale is Chinese, En otherwise.
func DetectLang() Lang {
	locale := systemLocale()
	if isChinese(locale) {
		return Zh
	}
	return En
}

// ParseLang maps a stored language code to a Lang, defaulting to the detected
// language when the value is empty or unrecognised.
func ParseLang(s string) Lang {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "zh", "zh-cn", "zh-hans", "chinese":
		return Zh
	case "en", "en-us", "english":
		return En
	}
	return DetectLang()
}

// isChinese reports whether a locale string designates Chinese.
func isChinese(locale string) bool {
	low := strings.ToLower(locale)
	return strings.HasPrefix(low, "zh")
}
