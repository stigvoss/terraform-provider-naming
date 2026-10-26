package provider

import (
	"strings"
	"testing"
)

func TestParseTemplate(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		keys    []string
		wantErr string
	}{
		{"simple", "{org}-{env}-{name}", []string{"org", "env", "name"}, ""},
		{"repeated key", "{a}-{a}-{b}", []string{"a", "b"}, ""},
		{"literal text", "pre-{a}-post", []string{"a"}, ""},
		{"unclosed", "{org-{env}", nil, "unbalanced '{'"},
		{"unclosed at end", "{org}-{env", nil, "unbalanced '{'"},
		{"stray close", "{org}-env}", nil, "unbalanced '}'"},
		{"empty key", "{org}-{}", nil, "empty placeholder"},
		{"blank key", "{ }", nil, "empty placeholder"},
		{"no placeholders", "static", nil, "no placeholders"},
		{"empty", "", nil, "no placeholders"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tpl, err := ParseTemplate(tc.in)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("want error containing %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := strings.Join(tpl.Keys(), ","); got != strings.Join(tc.keys, ",") {
				t.Fatalf("keys = %s, want %s", got, strings.Join(tc.keys, ","))
			}
		})
	}
}

func TestRender(t *testing.T) {
	static := map[string]string{"org": "acme", "env": "prod", "region": "weu"}
	tests := []struct {
		name     string
		template string
		call     map[string]string
		want     string
		wantErr  string
	}{
		{"ok", "{org}-{env}-{region}-{type}-{name}", map[string]string{"type": "rg", "name": "network"}, "acme-prod-weu-rg-network", ""},
		{"repeated placeholder", "{org}{org}-{name}", map[string]string{"name": "x"}, "acmeacme-x", ""},
		{"missing key", "{org}-{type}-{name}", map[string]string{"type": "rg"}, "", `"name"`},
		{"missing key mentions template", "{org}-{name}", map[string]string{}, "", "missing value"},
		{"extra key", "{org}-{name}", map[string]string{"name": "x", "nmae": "y"}, "", `"nmae" are not used`},
		{"override static", "{org}-{name}", map[string]string{"name": "x", "org": "evil"}, "", "cannot be overridden"},
		{"empty value allowed", "{org}-{name}", map[string]string{"name": ""}, "acme-", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tpl, err := ParseTemplate(tc.template)
			if err != nil {
				t.Fatal(err)
			}
			got, err := tpl.Render(static, tc.call)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("want error containing %q, got %v", tc.wantErr, err)
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

func TestUnusedKeys(t *testing.T) {
	tpl, _ := ParseTemplate("{org}-{name}")
	got := tpl.UnusedKeys([]string{"org", "zzz", "env"})
	if strings.Join(got, ",") != "env,zzz" {
		t.Fatalf("got %v", got)
	}
}
