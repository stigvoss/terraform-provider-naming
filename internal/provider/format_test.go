package provider

import (
	"strings"
	"testing"
)

func ip(n int) *int { return &n }

const kvTemplate = "{type}-{system-code}-{name}-{environment-code}-{location-code}-{discriminator}"

func kvScheme() map[string]ComponentSpec {
	return map[string]ComponentSpec{
		"type":             {MaxLength: ip(4)},
		"system-code":      {MaxLength: ip(3)},
		"name":             {OnTooLong: onTooLongTruncate},
		"environment-code": {Allowed: []string{"dev", "test", "prod"}},
		"location-code":    {MaxLength: ip(3)},
		"discriminator":    {MaxLength: ip(4)},
	}
}

func kvRules() Rules {
	return Rules{
		MaxLength:  ip(24),
		Fit:        "name",
		Components: map[string]ComponentSpec{"type": {MaxLength: ip(2)}},
	}
}

func kvCall(env, name string) map[string]string {
	return map[string]string{"type": "kv", "system-code": "lis", "name": name, "environment-code": env, "location-code": "weu", "discriminator": "b73a"}
}

func TestFormatKeyVault(t *testing.T) {
	tpl, err := ParseTemplate(kvTemplate)
	if err != nil {
		t.Fatal(err)
	}
	for env, want := range map[string]string{
		"dev":  "kv-lis-wha-dev-weu-b73a",
		"test": "kv-lis-wha-test-weu-b73a",
		"prod": "kv-lis-wha-prod-weu-b73a",
	} {
		got, err := tpl.Format(nil, kvCall(env, "wharf"), kvScheme(), kvRules())
		if err != nil {
			t.Fatalf("%s: %v", env, err)
		}
		if got != want {
			t.Errorf("%s: got %q, want %q", env, got, want)
		}
	}
}

func TestFormatKeyVaultErrors(t *testing.T) {
	tpl, _ := ParseTemplate(kvTemplate)
	tests := []struct {
		name    string
		call    map[string]string
		scheme  func() map[string]ComponentSpec
		rules   func() Rules
		wantErr string
	}{
		{"type too long for this rule", func() map[string]string { c := kvCall("dev", "wha"); c["type"] = "kvlt"; return c }(), kvScheme, kvRules, `"type"`},
		{"env not allowed", kvCall("staging", "wha"), kvScheme, kvRules, "not one of the allowed"},
		{"rule loosens scheme", kvCall("dev", "wha"), kvScheme, func() Rules {
			r := kvRules()
			r.Components = map[string]ComponentSpec{"type": {MaxLength: ip(6)}}
			return r
		}, "looser"},
		{"no room for name", kvCall("dev", "wha"), kvScheme, func() Rules { r := kvRules(); r.MaxLength = ip(20); return r }, "no room"},
		{"name too long errors by default", kvCall("dev", "wharf"), func() map[string]ComponentSpec {
			s := kvScheme()
			s["name"] = ComponentSpec{}
			return s
		}, kvRules, "the limit is 3"},
		{"fit needs max_length", kvCall("dev", "wha"), kvScheme, func() Rules { r := kvRules(); r.MaxLength = nil; return r }, "fit requires max_length"},
		{"fit not in template", kvCall("dev", "wha"), kvScheme, func() Rules { r := kvRules(); r.Fit = "nope"; return r }, "rules.fit"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tpl.Format(nil, tc.call, tc.scheme(), tc.rules())
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("want error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestFormatRules(t *testing.T) {
	tests := []struct {
		name     string
		template string
		static   map[string]string
		call     map[string]string
		comps    map[string]ComponentSpec
		rules    Rules
		want     string
		wantErr  string
	}{
		{
			name:     "storage account strips and lowercases",
			template: "{org}-{env}-{name}", static: map[string]string{"org": "Acme"}, call: map[string]string{"env": "Prod", "name": "My_Data"},
			rules: Rules{Lowercase: true, Remove: "[^a-z0-9]", MinLength: ip(3), MaxLength: ip(24)},
			want:  "acmeprodmydata",
		},
		{
			name:     "total too long without fit",
			template: "{org}-{name}", static: map[string]string{"org": "acme"}, call: map[string]string{"name": "abcdefghij"},
			rules:   Rules{MaxLength: ip(8)},
			wantErr: "the limit is 8",
		},
		{
			name:     "total too short",
			template: "{name}", call: map[string]string{"name": "a"},
			rules:   Rules{MinLength: ip(3)},
			wantErr: "the minimum is 3",
		},
		{
			name:     "component cap truncates",
			template: "{org}-{name}", static: map[string]string{"org": "acme"}, call: map[string]string{"name": "abcdefghij"},
			comps: map[string]ComponentSpec{"name": {MaxLength: ip(4), OnTooLong: onTooLongTruncate}},
			want:  "acme-abcd",
		},
		{
			name:     "hash suffix comes from the component only",
			template: "{env}-{name}", call: map[string]string{"env": "dev", "name": "abcdefghij"},
			comps: map[string]ComponentSpec{"name": {MaxLength: ip(5), OnTooLong: onTooLongTruncate, Hash: 2}},
			rules: Rules{},
			want:  "dev-abc" + sha2("abcdefghij", 2),
		},
		{
			name:     "unbounded component counts at actual length",
			template: "{env}-{name}", call: map[string]string{"env": "dev", "name": "abcdefghij"},
			comps: map[string]ComponentSpec{"name": {OnTooLong: onTooLongTruncate}},
			rules: Rules{MaxLength: ip(8), Fit: "name"},
			want:  "dev-abcd",
		},
		{
			name:     "pattern",
			template: "{env}", call: map[string]string{"env": "Dev"},
			comps:   map[string]ComponentSpec{"env": {Pattern: "^[a-z]+$"}},
			wantErr: "does not match",
		},
		{
			name:     "min_length on component",
			template: "{env}", call: map[string]string{"env": "d"},
			comps:   map[string]ComponentSpec{"env": {MinLength: ip(2)}},
			wantErr: "at least 2",
		},
		{
			name:     "component spec for unknown key",
			template: "{env}", call: map[string]string{"env": "d"},
			comps:   map[string]ComponentSpec{"nope": {MaxLength: ip(2)}},
			wantErr: "not used by the template",
		},
		{
			name:     "repeated fit component splits the budget",
			template: "{name}-{name}", call: map[string]string{"name": "abcdefgh"},
			comps: map[string]ComponentSpec{"name": {OnTooLong: onTooLongTruncate}},
			rules: Rules{MaxLength: ip(9), Fit: "name"},
			want:  "abcd-abcd",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tpl, err := ParseTemplate(tc.template)
			if err != nil {
				t.Fatal(err)
			}
			got, err := tpl.Format(tc.static, tc.call, tc.comps, tc.rules)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("want error containing %q, got %q, %v", tc.wantErr, got, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func sha2(s string, n int) string {
	v, _ := truncate("x", strings.Repeat("a", n+1), s, n+1, ComponentSpec{OnTooLong: onTooLongTruncate, Hash: n})
	return v[1:]
}
