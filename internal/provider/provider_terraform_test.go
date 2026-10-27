package provider

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

var testTFProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"naming": providerserver.NewProtocol6WithError(New("test")()),
}

const testTFConfig = `
locals {
  naming = provider::naming::scheme(
    "{org}-{env}-{region}-{type}-{name}",
    { org = "acme", env = "prod", region = "weu" },
  )
}

output "rg" {
  value = provider::naming::format(local.naming, { type = "rg", name = "network" })
}
`

func TestTerraformFormatFunction(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_8_0)},
		ProtoV6ProviderFactories: testTFProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testTFConfig,
				Check:  resource.TestCheckOutput("rg", "acme-prod-weu-rg-network"),
			},
		},
	})
}

func TestTerraformFormatFunctionErrors(t *testing.T) {
	cfg := func(call string) string {
		return `
locals {
  naming = provider::naming::scheme("{org}-{type}-{name}", { org = "acme" })
}
output "x" { value = provider::naming::format(local.naming, ` + call + `) }
`
	}
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_8_0)},
		ProtoV6ProviderFactories: testTFProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg(`{ type = "rg" }`), ExpectError: regexp.MustCompile(`missing\s+value`)},
			{Config: cfg(`{ type = "rg", name = "n", typo = "x" }`), ExpectError: regexp.MustCompile(`not used by the\s+template`)},
			{Config: cfg(`{ type = "rg", name = "n", org = "evil" }`), ExpectError: regexp.MustCompile(`cannot be\s+overridden`)},
			{Config: cfg(`{ type = null, name = "n" }`), ExpectError: regexp.MustCompile(`must not be\s+null`)},
		},
	})
}

func TestTerraformSchemeFunctionErrors(t *testing.T) {
	cfg := func(args string) string {
		return `output "x" { value = provider::naming::scheme(` + args + `) }`
	}
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_8_0)},
		ProtoV6ProviderFactories: testTFProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg(`"{org}-{name", {}`), ExpectError: regexp.MustCompile(`unbalanced`)},
			{Config: cfg(`"{org}-{name}", { orgg = "acme" }`), ExpectError: regexp.MustCompile(`not\s+used\s+by\s+the\s+template`)},
			{Config: cfg(`"static", {}`), ExpectError: regexp.MustCompile(`no\s+placeholders`)},
		},
	})
}

const testTFKeyVault = `
locals {
  naming = provider::naming::scheme(
    "{type}-{system}-{name}-{environment}-{region}-{discriminator}",
    { system = "lis", region = "weu", discriminator = "b73a" },
    {
      type          = { max_length = 4 }
      system        = { max_length = 3 }
      name          = { overflow = "truncate" }
      environment   = { allowed = ["dev", "test", "prod"] }
      region        = { max_length = 3 }
      discriminator = { max_length = 4 }
    },
    {
      by = "type"
      rules = {
        kv = { max_length = 24, fit = "name" }
        st = { lowercase = true, remove = "[^a-z0-9]", min_length = 3, max_length = 24, fit = "name" }
      }
    },
  )
}

output "kv_dev"  { value = provider::naming::format(local.naming, { type = "kv", name = "wharf", environment = "dev" }) }
output "kv_prod" { value = provider::naming::format(local.naming, { type = "kv", name = "wharf", environment = "prod" }) }
output "st_dev"  { value = provider::naming::format(local.naming, { type = "st", name = "datalakeextra", environment = "dev" }) }
output "rg"      { value = provider::naming::format(local.naming, { type = "rg", name = "network", environment = "dev" }) }
`

func TestTerraformKeyVault(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_8_0)},
		ProtoV6ProviderFactories: testTFProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testTFKeyVault,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("kv_dev", "kv-lis-wha-dev-weu-b73a"),
					resource.TestCheckOutput("kv_prod", "kv-lis-wha-prod-weu-b73a"),
					resource.TestCheckOutput("st_dev", "stlisdatalakedevweub73a"),
					// No rules for this type: only the scheme's constraints apply.
					resource.TestCheckOutput("rg", "rg-lis-network-dev-weu-b73a"),
				),
			},
		},
	})
}

func TestTerraformRulesErrors(t *testing.T) {
	cfg := func(rules string) string {
		return `output "x" { value = provider::naming::scheme(
  "{type}-{env}-{name}",
  {},
  { env = { allowed = ["dev", "prod"] }, type = { max_length = 4 } },
  ` + rules + `,
) }`
	}
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_8_0)},
		ProtoV6ProviderFactories: testTFProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      cfg(`{ by = "type", rules = { kv = { components = { type = { max_length = 9 } } } } }`),
				ExpectError: regexp.MustCompile(`looser`),
			},
			{
				Config:      cfg(`{ by = "type", rules = { kv = { fit = "name", max_length = 5 } } }`),
				ExpectError: regexp.MustCompile(`no\s+room`),
			},
			{
				Config:      cfg(`{ by = "type", rules = { kv = { lowercse = true } } }`),
				ExpectError: regexp.MustCompile(`unknown\s+field`),
			},
			{
				Config:      cfg(`{ by = "region", rules = {} }`),
				ExpectError: regexp.MustCompile(`not\s+used\s+by\s+the\s+template`),
			},
			{
				Config:      cfg(`{ rules = {} }`),
				ExpectError: regexp.MustCompile(`by\s+is\s+required`),
			},
			{
				// The key must itself be a valid value for the selecting component.
				Config:      cfg(`{ by = "type", rules = { keyvault = { max_length = 24 } } }`),
				ExpectError: regexp.MustCompile(`not\s+a\s+valid\s+type`),
			},
			{
				Config: `output "x" { value = provider::naming::format(
  provider::naming::scheme("{env}-{name}", {}, { env = { allowed = ["dev", "prod"] } }),
  { env = "staging", name = "a" },
) }`,
				ExpectError: regexp.MustCompile(`not\s+one\s+of\s+the\s+allowed`),
			},
			{
				Config:      `output "x" { value = provider::naming::scheme("{a}", {}, { b = { max_length = 3 } }) }`,
				ExpectError: regexp.MustCompile(`not\s+used\s+by\s+the\s+template`),
			},
		},
	})
}

// Values that are only known after apply must not fail the plan: constraints
// on them are checked once they are known.
func TestTerraformUnknownValues(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_8_0)},
		ProtoV6ProviderFactories: testTFProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// Known-wrong values are still rejected, with the value in the message.
				Config: `
locals {
  naming = provider::naming::scheme("{env}-{name}", { env = "staging" }, { env = { allowed = ["dev", "prod"] } })
}
output "x" { value = local.naming }
`,
				ExpectError: regexp.MustCompile(`not\s+one\s+of\s+the\s+allowed`),
			},
			{
				Config:      `output "x" { value = provider::naming::scheme("{a}", {}, { a = { hash = 99, overflow = "truncate" } }) }`,
				ExpectError: regexp.MustCompile(`between\s+0\s+and\s+64`),
			},
			{
				Config: `
resource "terraform_data" "env" {
  input = "dev"
}

locals {
  naming = provider::naming::scheme(
    "{type}-{env}-{name}",
    { env = terraform_data.env.output },
    { env = { allowed = ["dev", "prod"], max_length = 4 } },
    { by = "type", rules = { kv = { lowercase = true, max_length = 10, fit = "name" } } },
  )
}

output "static_unknown" {
  value = provider::naming::format(local.naming, { type = "kv", name = "a" })
}

output "variable_unknown" {
  value = provider::naming::format(
    provider::naming::scheme("{env}-{name}", {}, { env = { allowed = ["dev", "prod"] } }),
    { env = terraform_data.env.output, name = "b" },
  )
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("static_unknown", "kv-dev-a"),
					resource.TestCheckOutput("variable_unknown", "dev-b"),
				),
			},
		},
	})
}

func TestTerraformWithAndOverride(t *testing.T) {
	base := `
locals {
  base = provider::naming::scheme(
    "{type}-{system}-{name}-{env}-{region}-{discriminator}",
    { system = "wha", env = "dev" },
    {
      type          = { max_length = 4 }
      name          = { overflow = "truncate" }
      env           = { allowed = ["dev", "prod"] }
      region        = { max_length = 3 }
      discriminator = { max_length = 4 }
    },
    { by = "type", rules = { kv = { max_length = 24, fit = "name" } } },
  )
}
`
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_8_0)},
		ProtoV6ProviderFactories: testTFProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      base + `output "x" { value = provider::naming::with(local.base, { system = "other" }) }`,
				ExpectError: regexp.MustCompile(`already\s+a\s+static\s+value[\s\S]*override\(\)`),
			},
			{
				Config:      base + `output "x" { value = provider::naming::with(local.base, { nope = "x" }) }`,
				ExpectError: regexp.MustCompile(`not\s+a\s+placeholder`),
			},
			{
				Config:      base + `output "x" { value = provider::naming::with(local.base, { region = "europe" }) }`,
				ExpectError: regexp.MustCompile(`component\s+"region"\s+\(static\)[\s\S]*over\s+max_length\s+3`),
			},
			{
				Config:      base + `output "x" { value = provider::naming::override(local.base, { region = "weu" }) }`,
				ExpectError: regexp.MustCompile(`not\s+a\s+static\s+value[\s\S]*with\(\)`),
			},
			{
				Config:      base + `output "x" { value = provider::naming::override(local.base, { env = "staging" }) }`,
				ExpectError: regexp.MustCompile(`not\s+one\s+of\s+the\s+allowed`),
			},
			{
				// The last step must be valid: the test destroys with the last config.
				Config: base + `
locals {
  own    = provider::naming::with(local.base, { region = "weu", discriminator = "b73a" })
  shared = provider::naming::with(local.base, { region = "neu", discriminator = "c11d" })
  prod   = provider::naming::override(local.own, { env = "prod" })
}

resource "terraform_data" "region" {
  input = "weu"
}

output "own"    { value = provider::naming::format(local.own, { type = "kv", name = "wharf" }) }
output "shared" { value = provider::naming::format(local.shared, { type = "rg", name = "net" }) }
output "prod"   { value = provider::naming::format(local.prod, { type = "kv", name = "wharf" }) }
output "base_unchanged" { value = local.base.static.env }

# Unknown values are checked once they are known.
output "late" {
  value = provider::naming::format(
    provider::naming::with(local.base, { region = terraform_data.region.output, discriminator = "b73a" }),
    { type = "rg", name = "net" },
  )
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("own", "kv-wha-wha-dev-weu-b73a"),
					resource.TestCheckOutput("shared", "rg-wha-net-dev-neu-c11d"),
					// The rules and constraints of the base are kept.
					resource.TestCheckOutput("prod", "kv-wha-wha-prod-weu-b73a"),
					resource.TestCheckOutput("base_unchanged", "dev"),
					resource.TestCheckOutput("late", "rg-wha-net-dev-weu-b73a"),
				),
			},
		},
	})
}

// The configurations below are the README recipes, so they stay correct.
func TestTerraformRecipes(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_8_0)},
		ProtoV6ProviderFactories: testTFProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// A shared set with keys the template lacks is an error.
				Config: `
locals {
  all_components = {
    type = { max_length = 4 }
    site = { pattern = "^[a-z]{3}$" }
  }
  a = provider::naming::scheme("{type}-{name}", {}, local.all_components)
}
output "x" { value = local.a }
`,
				ExpectError: regexp.MustCompile(`component\s+"site"\s+is\s+not\s+used\s+by\s+the\s+template`),
			},
			{
				Config: `
locals {
  # Shared constraints, same placeholders.
  components = {
    type = { max_length = 4 }
    env  = { allowed = ["dev", "test", "prod"] }
  }
  own    = provider::naming::scheme("{type}-{name}-{env}", { env = "dev" }, local.components)
  shared = provider::naming::scheme("{type}-{name}-{env}", { env = "prod" }, local.components)

  # Shared constraints, different placeholders.
  all_components = {
    type = { max_length = 4 }
    site = { pattern = "^[a-z]{3}$" }
    role = { allowed = ["fleet", "convey", "lift"] }
  }
  subscription_keys = ["site", "role"]
  pair = provider::naming::scheme(
    "{site}-{role}",
    {},
    { for k, v in local.all_components : k => v if contains(local.subscription_keys, k) },
  )

  # No type prefix.
  subscription = provider::naming::scheme(
    "{site}-{slice}-{role}-{zone}",
    { slice = "wha" },
    {
      site = { pattern = "^[a-z]{3}$" }
      role = { allowed = ["fleet", "convey", "lift"] }
      zone = { pattern = "^[0-9]{2}(-[0-9]{2})*$" }
    },
  )
}

output "own"      { value = provider::naming::format(local.own, { type = "rg", name = "net" }) }
output "shared"   { value = provider::naming::format(local.shared, { type = "rg", name = "net" }) }
output "pair"     { value = provider::naming::format(local.pair, { site = "las", role = "lift" }) }
output "sub_one"  { value = provider::naming::format(local.subscription, { site = "las", role = "fleet", zone = "04" }) }
output "sub_two"  { value = provider::naming::format(local.subscription, { site = "las", role = "lift", zone = "04-06" }) }
output "sub_up"   { value = upper(provider::naming::format(local.subscription, { site = "las", role = "fleet", zone = "04" })) }
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("own", "rg-net-dev"),
					resource.TestCheckOutput("shared", "rg-net-prod"),
					resource.TestCheckOutput("pair", "las-lift"),
					resource.TestCheckOutput("sub_one", "las-wha-fleet-04"),
					resource.TestCheckOutput("sub_two", "las-wha-lift-04-06"),
					resource.TestCheckOutput("sub_up", "LAS-WHA-FLEET-04"),
				),
			},
		},
	})
}

// The files under examples/ are what the generated docs show, so run them.
func TestTerraformExamples(t *testing.T) {
	outputs := map[string]map[string]string{
		"scheme":   {"key_vault_name": "kv-lis-wha-dev-weu-b73a", "storage_account_name": "stlisdatalakedevweub73a", "resource_group_name": "rg-lis-network-dev-weu-b73a"},
		"format":   {"resource_group_name": "acme-prod-weu-rg-network"},
		"with":     {"own_resource_group": "rg-wha-net-dev-weu-b73a", "shared_resource_group": "rg-wha-net-dev-neu-c11d"},
		"override": {"prod_resource_group": "rg-wha-net-prod-weu"},
	}
	for name, want := range outputs {
		t.Run(name, func(t *testing.T) {
			cfg, err := os.ReadFile(filepath.Join("..", "..", "examples", "functions", name, "function.tf"))
			if err != nil {
				t.Fatal(err)
			}
			var checks []resource.TestCheckFunc
			for k, v := range want {
				checks = append(checks, resource.TestCheckOutput(k, v))
			}
			resource.UnitTest(t, resource.TestCase{
				TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_8_0)},
				ProtoV6ProviderFactories: testTFProtoV6ProviderFactories,
				Steps:                    []resource.TestStep{{Config: string(cfg), Check: resource.ComposeAggregateTestCheckFunc(checks...)}},
			})
		})
	}
}
