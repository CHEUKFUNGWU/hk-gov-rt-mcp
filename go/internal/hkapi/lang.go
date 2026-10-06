package hkapi

// Lang mirrors the TS side: tc=繁體, sc=简体, en=English.
type Lang string

const (
	TC Lang = "tc"
	SC Lang = "sc"
	EN Lang = "en"
)

// Normalize validates a raw lang string, defaulting to tc.
func NormalizeLang(s string) Lang {
	switch Lang(s) {
	case TC, SC, EN:
		return Lang(s)
	}
	return TC
}

// Suffix returns the JSON field suffix used by HK open data (name_tc etc.).
func (l Lang) Suffix() string { return string(l) }

// Pick reads base+"_"+lang (falling back to _en) from a decoded JSON object.
func Pick(m map[string]any, base string, lang Lang) string {
	if v, ok := m[base+"_"+string(lang)].(string); ok && v != "" {
		return v
	}
	if v, ok := m[base+"_en"].(string); ok {
		return v
	}
	return ""
}

// Str reads a string field.
func Str(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// Arr reads an array field.
func Arr(m map[string]any, key string) []any {
	if v, ok := m[key].([]any); ok {
		return v
	}
	return nil
}

// Obj reads an object field.
func Obj(m map[string]any, key string) map[string]any {
	if v, ok := m[key].(map[string]any); ok {
		return v
	}
	return nil
}

// AsObjList converts an []any of objects.
func AsObjList(items []any) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		if o, ok := it.(map[string]any); ok {
			out = append(out, o)
		}
	}
	return out
}
