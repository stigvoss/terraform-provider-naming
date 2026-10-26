package provider

import (
	"fmt"
	"sort"
	"strings"
)

// Template is a parsed naming template such as "{org}-{env}-{name}".
type Template struct {
	raw   string
	parts []part
	keys  []string
}

type part struct {
	text  string
	isKey bool
}

// ParseTemplate splits a template into literal text and {key} placeholders.
// Braces must be balanced, placeholders cannot be nested or empty, and a
// stray "}" is an error. There is no escape syntax.
func ParseTemplate(raw string) (*Template, error) {
	t := &Template{raw: raw}
	seen := map[string]bool{}
	var lit strings.Builder

	for i := 0; i < len(raw); i++ {
		switch raw[i] {
		case '{':
			end := strings.IndexAny(raw[i+1:], "{}")
			if end < 0 || raw[i+1+end] == '{' {
				return nil, fmt.Errorf("unbalanced '{' at position %d in template %q", i, raw)
			}
			key := raw[i+1 : i+1+end]
			if strings.TrimSpace(key) == "" {
				return nil, fmt.Errorf("empty placeholder at position %d in template %q", i, raw)
			}
			if lit.Len() > 0 {
				t.parts = append(t.parts, part{text: lit.String()})
				lit.Reset()
			}
			t.parts = append(t.parts, part{text: key, isKey: true})
			if !seen[key] {
				seen[key] = true
				t.keys = append(t.keys, key)
			}
			i += end + 1
		case '}':
			return nil, fmt.Errorf("unbalanced '}' at position %d in template %q", i, raw)
		default:
			lit.WriteByte(raw[i])
		}
	}
	if lit.Len() > 0 {
		t.parts = append(t.parts, part{text: lit.String()})
	}
	if len(t.keys) == 0 {
		return nil, fmt.Errorf("template %q contains no placeholders", raw)
	}
	return t, nil
}

// Keys returns the distinct placeholder names in order of first appearance.
func (t *Template) Keys() []string { return t.keys }

// Has reports whether the template uses the given placeholder.
func (t *Template) Has(key string) bool {
	for _, k := range t.keys {
		if k == key {
			return true
		}
	}
	return false
}

// UnusedKeys returns the given keys that the template does not reference, sorted.
func (t *Template) UnusedKeys(keys []string) []string {
	var out []string
	for _, k := range keys {
		if !t.Has(k) {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// Render fills the template from static and call values. A call key that
// shadows a static key, a call key the template does not use, and any
// placeholder left without a value are all errors.
func (t *Template) Render(static, call map[string]string) (string, error) {
	var shadowed, extra, missing []string
	for k := range call {
		if _, ok := static[k]; ok {
			shadowed = append(shadowed, k)
		} else if !t.Has(k) {
			extra = append(extra, k)
		}
	}
	for _, k := range t.keys {
		_, inStatic := static[k]
		_, inCall := call[k]
		if !inStatic && !inCall {
			missing = append(missing, k)
		}
	}
	switch {
	case len(shadowed) > 0:
		sort.Strings(shadowed)
		return "", fmt.Errorf("key(s) %s are set by the provider's static block and cannot be overridden", quoteAll(shadowed))
	case len(extra) > 0:
		sort.Strings(extra)
		return "", fmt.Errorf("key(s) %s are not used by the template %q", quoteAll(extra), t.raw)
	case len(missing) > 0:
		return "", fmt.Errorf("missing value for placeholder(s) %s in template %q", quoteAll(missing), t.raw)
	}

	var b strings.Builder
	for _, p := range t.parts {
		if !p.isKey {
			b.WriteString(p.text)
			continue
		}
		if v, ok := call[p.text]; ok {
			b.WriteString(v)
		} else {
			b.WriteString(static[p.text])
		}
	}
	return b.String(), nil
}

func quoteAll(keys []string) string {
	q := make([]string, len(keys))
	for i, k := range keys {
		q[i] = fmt.Sprintf("%q", k)
	}
	return strings.Join(q, ", ")
}
