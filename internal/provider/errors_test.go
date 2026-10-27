package provider

import (
	"errors"
	"testing"
)

func TestErrorMessages(t *testing.T) {
	tpl, err := ParseTemplate("{type}-{name}-{env}")
	if err != nil {
		t.Fatal(err)
	}
	comps := map[string]ComponentSpec{
		"type": {Pattern: "^[a-z]+$"},
		"name": {MinLength: ip(2), MaxLength: ip(5)},
		"env":  {Allowed: []string{"dev", "prod"}},
	}
	ruleset := Ruleset{By: "type", Rules: map[string]Rules{"kv": {MaxLength: ip(10), Fit: "name"}}}

	tests := []struct {
		name    string
		static  map[string]string
		call    map[string]string
		rules   Rules
		want    string
		wantArg int64
	}{
		{
			name: "allowed, variable",
			call: map[string]string{"type": "rg", "name": "net", "env": "staging"},
			want: `component "env" (variable): "staging" is not one of the allowed values "dev", "prod"`, wantArg: 1,
		},
		{
			name:   "allowed, static is reported against the scheme",
			static: map[string]string{"env": "staging"},
			call:   map[string]string{"type": "rg", "name": "net"},
			want:   `component "env" (static): "staging" is not one of the allowed values "dev", "prod"`, wantArg: 0,
		},
		{
			name: "pattern",
			call: map[string]string{"type": "RG", "name": "net", "env": "dev"},
			want: `component "type" (variable): "RG" does not match the pattern "^[a-z]+$"`, wantArg: 1,
		},
		{
			name: "min_length",
			call: map[string]string{"type": "rg", "name": "n", "env": "dev"},
			want: `component "name" (variable): "n" is 1 character, below min_length 2`, wantArg: 1,
		},
		{
			name: "max_length",
			call: map[string]string{"type": "rg", "name": "network", "env": "dev"},
			want: `component "name" (variable): "network" is 7 characters, over max_length 5`, wantArg: 1,
		},
		{
			name:  "fit budget names the rules",
			call:  map[string]string{"type": "kv", "name": "wharf", "env": "dev"},
			rules: ruleset.For("kv"),
			want:  `component "name" (variable): "wharf" is 5 characters, over the 2 characters left for it by fit under max_length 10 in rules for "kv"`, wantArg: 1,
		},
		{
			name:  "total length names the rules",
			call:  map[string]string{"type": "kv", "name": "ab", "env": "prod"},
			rules: Rules{name: "kv", MaxLength: ip(8)},
			want:  `the name "kv-ab-prod" is 10 characters, over max_length 8 in rules for "kv"`, wantArg: 1,
		},
		{
			name: "missing variable",
			call: map[string]string{"type": "rg", "env": "dev"},
			want: `missing value for placeholder(s) "name" in template "{type}-{name}-{env}"`, wantArg: 1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tpl.Format(tc.static, tc.call, comps, tc.rules)
			if err == nil {
				t.Fatal("want an error")
			}
			if err.Error() != tc.want {
				t.Fatalf("message\n got: %s\nwant: %s", err, tc.want)
			}
			if got := argOf(err, -1); got != tc.wantArg {
				t.Fatalf("argument = %d, want %d", got, tc.wantArg)
			}
		})
	}
}

func TestWithScheme(t *testing.T) {
	err := errors.New(`component "env" (variable): "x" is not one of the allowed values "dev"`)
	if got, want := withScheme(err, "{env}-{name}"), `component "env" (variable): "x" is not one of the allowed values "dev" (scheme "{env}-{name}")`; got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
	// A message that already quotes the template is not repeated.
	err = errors.New(`missing value for placeholder(s) "name" in template "{env}-{name}"`)
	if got := withScheme(err, "{env}-{name}"); got != err.Error() {
		t.Fatalf("template repeated: %s", got)
	}
}

func TestPlural(t *testing.T) {
	if chars(1) != "1 character" || chars(0) != "0 characters" || chars(24) != "24 characters" {
		t.Fatal("unexpected plural forms")
	}
}
