package provider

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

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

// CheckValue validates one value against its spec and applies on_too_long.
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
		if re := regexp.MustCompile(s.Pattern); !re.MatchString(value) {
			return "", fmt.Errorf("%q is %q, which does not match the pattern %q", key, value, s.Pattern)
		}
	}
	if s.MinLength != nil && len(value) < *s.MinLength {
		return "", fmt.Errorf("%q is %q (%d characters), the scheme requires at least %d", key, value, len(value), *s.MinLength)
	}
	if s.MaxLength != nil && len(value) > *s.MaxLength {
		return truncate(key, value, value, *s.MaxLength, s)
	}
	return value, nil
}

// truncate shortens v to limit characters per the spec's on_too_long. orig is
// the value the optional hash is computed from, so a trimmed component stays
// the same regardless of the other components.
func truncate(key, v, orig string, limit int, s ComponentSpec) (string, error) {
	if s.OnTooLong != onTooLongTruncate {
		return "", fmt.Errorf("%q is %q (%d characters), the limit is %d", key, v, len(v), limit)
	}
	if s.Hash > 0 {
		sum := sha256.Sum256([]byte(orig))
		h := hex.EncodeToString(sum[:])
		if s.Hash >= limit {
			return h[:limit], nil
		}
		return v[:limit-s.Hash] + h[:s.Hash], nil
	}
	return v[:limit], nil
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
	vals, err := t.merge(static, call)
	if err != nil {
		return "", err
	}

	var remove *regexp.Regexp
	if rules.Remove != "" {
		remove = regexp.MustCompile(rules.Remove)
	}
	transform := func(s string) string {
		if rules.Lowercase {
			s = strings.ToLower(s)
		}
		if remove != nil {
			s = remove.ReplaceAllString(s, "")
		}
		return s
	}

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
	if rules.MaxLength != nil && len(out) > *rules.MaxLength {
		return "", fmt.Errorf("name %q is %d characters, the limit is %d", out, len(out), *rules.MaxLength)
	}
	if rules.MinLength != nil && len(out) < *rules.MinLength {
		return "", fmt.Errorf("name %q is %d characters, the minimum is %d", out, len(out), *rules.MinLength)
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

// fit shrinks the fit component so the name stays within rules.MaxLength.
// The budget is computed from the declared bounds of the other components,
// not their actual lengths, so the fit component is trimmed identically in
// every environment. A component without a bound counts at its actual length.
func (t *Template) fit(vals, orig map[string]string, specs map[string]ComponentSpec, rules Rules, transform func(string) string) error {
	fixed, occurrences := 0, 0
	for _, p := range t.parts {
		switch {
		case !p.isKey:
			fixed += len(transform(p.text))
		case p.text == rules.Fit:
			occurrences++
		default:
			if b, ok := specs[p.text].Bound(); ok {
				fixed += b
			} else {
				fixed += len(vals[p.text])
			}
		}
	}
	budget := (*rules.MaxLength - fixed) / occurrences
	spec := specs[rules.Fit]
	limit := budget
	if b, ok := spec.Bound(); ok && b < limit {
		limit = b
	}
	floor := 1
	if spec.MinLength != nil && *spec.MinLength > floor {
		floor = *spec.MinLength
	}
	if limit < floor {
		return fmt.Errorf("no room for %q: the other components use %d of %d characters, leaving %d per occurrence but at least %d is needed",
			rules.Fit, fixed, *rules.MaxLength, budget, floor)
	}
	v := vals[rules.Fit]
	if len(v) > limit {
		short, err := truncate(rules.Fit, v, orig[rules.Fit], limit, spec)
		if err != nil {
			return fmt.Errorf("%w (budget %d from max_length %d)", err, budget, *rules.MaxLength)
		}
		vals[rules.Fit] = short
	}
	return nil
}

// CheckRules validates rules against a scheme without any values: component
// overrides must tighten, and the fit component must have room even when every
// other component is as short as it can be. Unbounded components count as
// zero, so this only reports budgets that are impossible, never ones that
// merely might be.
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
	var remove *regexp.Regexp
	if rules.Remove != "" {
		remove = regexp.MustCompile(rules.Remove)
	}
	fixed, occurrences := 0, 0
	for _, p := range t.parts {
		switch {
		case !p.isKey:
			lit := p.text
			if rules.Lowercase {
				lit = strings.ToLower(lit)
			}
			if remove != nil {
				lit = remove.ReplaceAllString(lit, "")
			}
			fixed += len(lit)
		case p.text == rules.Fit:
			occurrences++
		default:
			if b, ok := specs[p.text].Bound(); ok {
				fixed += b
			}
		}
	}
	budget := (*rules.MaxLength - fixed) / occurrences
	floor := 1
	if m := specs[rules.Fit].MinLength; m != nil && *m > floor {
		floor = *m
	}
	if budget < floor {
		return fmt.Errorf("no room for %q: the other components use up to %d of %d characters, leaving %d per occurrence but at least %d is needed",
			rules.Fit, fixed, *rules.MaxLength, budget, floor)
	}
	return nil
}
