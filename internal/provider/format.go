package provider

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

// length counts characters, not bytes, so limits mean the same for any input.
func length(s string) int { return utf8.RuneCountInString(s) }

// cut keeps the first n characters of s.
func cut(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if n >= length(s) {
		return s
	}
	return string([]rune(s)[:n])
}

// CheckSpecs reports components that name a placeholder the template lacks
// and invalid specs. It is safe to call without values.
func (t *Template) CheckSpecs(specs map[string]ComponentSpec) error {
	for k, s := range specs {
		if !t.Has(k) {
			return fmt.Errorf("component %q is not used by the template %q", k, t.raw)
		}
		if err := s.Validate(); err != nil {
			return fmt.Errorf("component %q: %w", k, err)
		}
	}
	return nil
}

// CheckValue validates one value against its spec and applies overflow.
func CheckValue(key, value string, s ComponentSpec) (string, error) {
	if len(s.Allowed) > 0 {
		found := false
		for _, a := range s.Allowed {
			if a == value {
				found = true
				break
			}
		}
		if !found {
			return "", fmt.Errorf("%q is %q, which is not one of the allowed values %s", key, value, quoteAll(s.Allowed))
		}
	}
	if s.Pattern != "" {
		if !mustRegexp(s.Pattern).MatchString(value) {
			return "", fmt.Errorf("%q is %q, which does not match the pattern %q", key, value, s.Pattern)
		}
	}
	if s.MinLength != nil && length(value) < *s.MinLength {
		return "", fmt.Errorf("%q is %q (%d characters), the scheme requires at least %d", key, value, length(value), *s.MinLength)
	}
	if s.MaxLength != nil && length(value) > *s.MaxLength {
		return truncate(key, value, value, *s.MaxLength, s)
	}
	return value, nil
}

// truncate shortens v to limit characters per the spec's overflow. orig is
// the value the optional hash is computed from, so a trimmed component stays
// the same regardless of the other components.
func truncate(key, v, orig string, limit int, s ComponentSpec) (string, error) {
	if s.Overflow != overflowTruncate {
		return "", fmt.Errorf("%q is %q (%d characters), the limit is %d", key, v, length(v), limit)
	}
	if s.Hash > 0 {
		sum := sha256.Sum256([]byte(orig))
		h := hex.EncodeToString(sum[:])
		if s.Hash >= limit {
			return h[:limit], nil
		}
		return cut(v, limit-s.Hash) + h[:s.Hash], nil
	}
	return cut(v, limit), nil
}

// Format renders the template under the scheme's component specs and the
// type's rules. The pipeline is: validate components, apply lowercase and
// remove, shrink the fit component into its budget, render, check the total.
func (t *Template) Format(static, call map[string]string, comps map[string]ComponentSpec, rules Rules) (string, error) {
	if err := t.CheckSpecs(comps); err != nil {
		return "", err
	}
	if err := rules.Validate(); err != nil {
		return "", err
	}
	specs, err := t.effectiveSpecs(comps, rules)
	if err != nil {
		return "", err
	}
	if err := t.CheckStaticKeys(keysOf(static)); err != nil {
		return "", err
	}
	vals, err := t.merge(static, call)
	if err != nil {
		return "", err
	}
	transform := rules.transformer()

	orig := map[string]string{}
	for _, k := range t.keys {
		orig[k] = vals[k]
		v, err := CheckValue(k, vals[k], specs[k])
		if err != nil {
			return "", err
		}
		vals[k] = transform(v)
	}

	if rules.Fit != "" {
		if err := t.fit(vals, orig, specs, rules, transform); err != nil {
			return "", err
		}
	}

	var b strings.Builder
	for _, p := range t.parts {
		if p.isKey {
			b.WriteString(vals[p.text])
		} else {
			b.WriteString(transform(p.text))
		}
	}
	out := b.String()
	if rules.MaxLength != nil && length(out) > *rules.MaxLength {
		return "", fmt.Errorf("name %q is %d characters, the limit is %d", out, length(out), *rules.MaxLength)
	}
	if rules.MinLength != nil && length(out) < *rules.MinLength {
		return "", fmt.Errorf("name %q is %d characters, the minimum is %d", out, length(out), *rules.MinLength)
	}
	return out, nil
}

// effectiveSpecs overlays the type's component overrides on the scheme's specs.
func (t *Template) effectiveSpecs(comps map[string]ComponentSpec, rules Rules) (map[string]ComponentSpec, error) {
	out := make(map[string]ComponentSpec, len(t.keys))
	for _, k := range t.keys {
		out[k] = comps[k]
	}
	for k, over := range rules.Components {
		if !t.Has(k) {
			return nil, fmt.Errorf("rules.components.%s is not used by the template %q", k, t.raw)
		}
		tight, err := tighten(k, out[k], over)
		if err != nil {
			return nil, err
		}
		out[k] = tight
	}
	if rules.Fit != "" && !t.Has(rules.Fit) {
		return nil, fmt.Errorf("rules.fit %q is not used by the template %q", rules.Fit, t.raw)
	}
	return out, nil
}

// fitLimit works out how many characters each occurrence of the fit component
// may use. The other components count at their declared bound; one without a
// bound counts as unbounded(key) (its actual length when formatting, zero when
// checking rules without values).
func (t *Template) fitLimit(specs map[string]ComponentSpec, rules Rules, transform func(string) string, unbounded func(key string) int) (limit, floor int, err error) {
	fixed, occurrences := 0, 0
	for _, p := range t.parts {
		switch {
		case !p.isKey:
			fixed += length(transform(p.text))
		case p.text == rules.Fit:
			occurrences++
		default:
			if b, ok := specs[p.text].Bound(); ok {
				fixed += b
			} else {
				fixed += unbounded(p.text)
			}
		}
	}
	budget := (*rules.MaxLength - fixed) / occurrences
	spec := specs[rules.Fit]
	limit = budget
	if b, ok := spec.Bound(); ok && b < limit {
		limit = b
	}
	floor = 1
	if spec.MinLength != nil && *spec.MinLength > floor {
		floor = *spec.MinLength
	}
	if limit < floor {
		return limit, floor, fmt.Errorf("no room for %q: the other components use %d of %d characters, leaving %d per occurrence but at least %d is needed",
			rules.Fit, fixed, *rules.MaxLength, budget, floor)
	}
	return limit, floor, nil
}

// fit shrinks the fit component so the name stays within rules.MaxLength.
// The budget is computed from the declared bounds of the other components,
// not their actual lengths, so the fit component is trimmed identically in
// every environment.
func (t *Template) fit(vals, orig map[string]string, specs map[string]ComponentSpec, rules Rules, transform func(string) string) error {
	limit, _, err := t.fitLimit(specs, rules, transform, func(k string) int { return length(vals[k]) })
	if err != nil {
		return err
	}
	v := vals[rules.Fit]
	if length(v) > limit {
		short, err := truncate(rules.Fit, v, orig[rules.Fit], limit, specs[rules.Fit])
		if err != nil {
			return fmt.Errorf("%w (limit calculated from max_length %d)", err, *rules.MaxLength)
		}
		vals[rules.Fit] = short
	}
	return nil
}

// CheckRules validates rules against a scheme without any values: component
// overrides must tighten, and the fit component must have room even when every
// unbounded component is empty. This only reports budgets that are impossible,
// never ones that merely might be.
func (t *Template) CheckRules(comps map[string]ComponentSpec, rules Rules) error {
	if err := t.CheckSpecs(comps); err != nil {
		return err
	}
	if err := rules.Validate(); err != nil {
		return err
	}
	specs, err := t.effectiveSpecs(comps, rules)
	if err != nil {
		return err
	}
	if rules.Fit == "" {
		return nil
	}
	_, _, err = t.fitLimit(specs, rules, rules.transformer(), func(string) int { return 0 })
	return err
}

// CheckStaticKeys reports static keys the template does not use. Only the
// keys are needed, so it works while the values are still unknown.
func (t *Template) CheckStaticKeys(keys []string) error {
	if unused := t.UnusedKeys(keys); len(unused) > 0 {
		return fmt.Errorf("static key(s) %s are not used by the template %q", quoteAll(unused), t.raw)
	}
	return nil
}

func keysOf(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// transformer returns the lowercase and remove steps as one function.
func (r Rules) transformer() func(string) string {
	return func(s string) string {
		if r.Lowercase {
			s = strings.ToLower(s)
		}
		if r.Remove != "" {
			s = mustRegexp(r.Remove).ReplaceAllString(s, "")
		}
		return s
	}
}

// CheckRuleset validates a scheme's per-type rules without any values: the
// selecting placeholder exists, every rule is valid against the scheme's
// components, and every selector value is itself a valid value for that
// placeholder (under the tightening of its own rule).
func (t *Template) CheckRuleset(comps map[string]ComponentSpec, rs Ruleset) error {
	if rs.By == "" {
		return nil
	}
	if !t.Has(rs.By) {
		return fmt.Errorf("rules.by %q is not used by the template %q", rs.By, t.raw)
	}
	keys := make([]string, 0, len(rs.Rules))
	for k := range rs.Rules {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		rule := rs.Rules[k]
		if err := t.CheckRules(comps, rule); err != nil {
			return fmt.Errorf("rules for %q: %w", k, err)
		}
		specs, _ := t.effectiveSpecs(comps, rule)
		if _, err := CheckValue(rs.By, k, specs[rs.By]); err != nil {
			return fmt.Errorf("rules for %q: the key is not a valid %s: %w", k, rs.By, err)
		}
	}
	return nil
}
