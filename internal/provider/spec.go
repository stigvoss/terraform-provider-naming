package provider

import (
	"fmt"
	"regexp"
	"sort"
	"sync"
)

// maxHash is the number of hex characters in the SHA-256 digest hash draws from.
const maxHash = 64

var regexpCache sync.Map

// mustRegexp compiles a pattern once. Patterns are validated before use, so
// it does not fail for specs and rules that passed Validate.
func mustRegexp(pattern string) *regexp.Regexp {
	if re, ok := regexpCache.Load(pattern); ok {
		return re.(*regexp.Regexp)
	}
	re := regexp.MustCompile(pattern)
	regexpCache.Store(pattern, re)
	return re
}

const (
	overflowError    = "error"
	overflowTruncate = "truncate"
)

// ComponentSpec constrains one template component. Every field is optional;
// the zero value means unlimited.
type ComponentSpec struct {
	MinLength *int
	MaxLength *int
	Allowed   []string
	Pattern   string // regular expression the whole value must match
	Overflow  string // overflowError (default) or overflowTruncate
	Hash      int    // hex characters of a hash of the original value appended when truncating
}

// Rules are per-resource-type constraints applied to the whole name.
type Rules struct {
	Lowercase bool
	Remove    string // regular expression; matches are deleted from the name
	MinLength *int
	MaxLength *int
	Fit       string // component that is shrunk to make the name fit MaxLength
	// Components tightens (never loosens) the scheme's component specs for this type.
	Components map[string]ComponentSpec
}

// Validate checks the spec is self-consistent.
func (c ComponentSpec) Validate() error {
	if c.MinLength != nil && *c.MinLength < 0 {
		return fmt.Errorf("min_length must not be negative")
	}
	if c.MaxLength != nil && *c.MaxLength < 0 {
		return fmt.Errorf("max_length must not be negative")
	}
	if c.MinLength != nil && c.MaxLength != nil && *c.MinLength > *c.MaxLength {
		return fmt.Errorf("min_length %d is greater than max_length %d", *c.MinLength, *c.MaxLength)
	}
	if c.Hash < 0 || c.Hash > maxHash {
		return fmt.Errorf("hash must be between 0 and %d", maxHash)
	}
	if c.Hash > 0 && c.Overflow != overflowTruncate {
		return fmt.Errorf("hash only applies with overflow = %q", overflowTruncate)
	}
	switch c.Overflow {
	case "", overflowError, overflowTruncate:
	default:
		return fmt.Errorf("overflow must be %q or %q, got %q", overflowError, overflowTruncate, c.Overflow)
	}
	if c.Pattern != "" {
		if _, err := regexp.Compile(c.Pattern); err != nil {
			return fmt.Errorf("invalid pattern: %w", err)
		}
	}
	return nil
}

// Bound returns the largest length a valid value can have, from max_length
// and the longest allowed value. ok is false when the component is unlimited.
func (c ComponentSpec) Bound() (n int, ok bool) {
	if c.MaxLength != nil {
		n, ok = *c.MaxLength, true
	}
	if len(c.Allowed) > 0 {
		longest := 0
		for _, a := range c.Allowed {
			if n := length(a); n > longest {
				longest = n
			}
		}
		if !ok || longest < n {
			n, ok = longest, true
		}
	}
	return n, ok
}

// Validate checks the rules are self-consistent.
func (r Rules) Validate() error {
	if r.Remove != "" {
		if _, err := regexp.Compile(r.Remove); err != nil {
			return fmt.Errorf("invalid remove pattern: %w", err)
		}
	}
	if r.MinLength != nil && r.MaxLength != nil && *r.MinLength > *r.MaxLength {
		return fmt.Errorf("min_length %d is greater than max_length %d", *r.MinLength, *r.MaxLength)
	}
	if r.Fit != "" && r.MaxLength == nil {
		return fmt.Errorf("fit requires max_length")
	}
	return nil
}

// tighten applies a rule-level override to a scheme-level spec. The override
// may only narrow it.
func tighten(key string, base, over ComponentSpec) (ComponentSpec, error) {
	out := base
	if over.MaxLength != nil {
		if b, ok := base.Bound(); ok && *over.MaxLength > b {
			return out, fmt.Errorf("components.%s.max_length %d is looser than the scheme's limit of %d", key, *over.MaxLength, b)
		}
		out.MaxLength = over.MaxLength
	}
	if over.MinLength != nil {
		if base.MinLength != nil && *over.MinLength < *base.MinLength {
			return out, fmt.Errorf("components.%s.min_length %d is looser than the scheme's minimum of %d", key, *over.MinLength, *base.MinLength)
		}
		out.MinLength = over.MinLength
	}
	if len(over.Allowed) > 0 {
		if len(base.Allowed) > 0 {
			ok := map[string]bool{}
			for _, a := range base.Allowed {
				ok[a] = true
			}
			var extra []string
			for _, a := range over.Allowed {
				if !ok[a] {
					extra = append(extra, a)
				}
			}
			if len(extra) > 0 {
				sort.Strings(extra)
				return out, fmt.Errorf("components.%s.allowed contains %s which the scheme does not allow", key, quoteAll(extra))
			}
		}
		out.Allowed = over.Allowed
	}
	if err := out.Validate(); err != nil {
		return out, fmt.Errorf("components.%s: %w", key, err)
	}
	return out, nil
}
