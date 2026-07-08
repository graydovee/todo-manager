package i18n

import (
	"sort"
	"testing"
)

// TestLocaleParity ensures en and zh have identical key sets, mirroring the web
// frontend's locale-parity invariant.
func TestLocaleParity(t *testing.T) {
	enKeys := keySet(en)
	zhKeys := keySet(zh)

	missingInZh := diff(enKeys, zhKeys)
	missingInEn := diff(zhKeys, enKeys)

	if len(missingInZh) > 0 {
		t.Errorf("keys present in en but missing in zh: %v", missingInZh)
	}
	if len(missingInEn) > 0 {
		t.Errorf("keys present in zh but missing in en: %v", missingInEn)
	}
}

func TestInterpolation(t *testing.T) {
	tr := &Translator{lang: En, dict: en}
	got := tr.T("common.items", "count", 5)
	if got != "5 items" {
		t.Errorf("en items = %q, want %q", got, "5 items")
	}
	tr.SetLang(Zh)
	got = tr.T("common.items", "count", 12)
	if got != "12 条" {
		t.Errorf("zh items = %q, want %q", got, "12 条")
	}
}

func TestFallback(t *testing.T) {
	tr := &Translator{lang: En, dict: en}
	if got := tr.T("nonexistent.key"); got != "nonexistent.key" {
		t.Errorf("missing key = %q, want the key itself", got)
	}
}

func TestSetLang(t *testing.T) {
	tr := &Translator{lang: En, dict: en}
	if got := tr.T("common.cancel"); got != "Cancel" {
		t.Errorf("en cancel = %q", got)
	}
	tr.SetLang(Zh)
	if tr.Lang() != Zh {
		t.Errorf("Lang() = %q, want zh", tr.Lang())
	}
	if got := tr.T("common.cancel"); got != "取消" {
		t.Errorf("zh cancel = %q, want %q", got, "取消")
	}
}

func keySet(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// diff returns elements in a that are not in b (a, b must be sorted).
func diff(a, b []string) []string {
	var out []string
	i, j := 0, 0
	for i < len(a) {
		switch {
		case j >= len(b) || a[i] < b[j]:
			out = append(out, a[i])
			i++
		case a[i] == b[j]:
			i++
			j++
		default:
			j++
		}
	}
	return out
}
