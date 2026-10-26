package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"naming": providerserver.NewProtocol6WithError(New("test")()),
}

const testAccConfig = `
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

func TestAccFormatFunction(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_8_0)},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccConfig,
				Check:  resource.TestCheckOutput("rg", "acme-prod-weu-rg-network"),
			},
		},
	})
}

func TestAccFormatFunctionErrors(t *testing.T) {
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
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg(`{ type = "rg" }`), ExpectError: regexp.MustCompile(`missing\s+value`)},
			{Config: cfg(`{ type = "rg", name = "n", typo = "x" }`), ExpectError: regexp.MustCompile(`not used by the\s+template`)},
			{Config: cfg(`{ type = "rg", name = "n", org = "evil" }`), ExpectError: regexp.MustCompile(`cannot be\s+overridden`)},
			{Config: cfg(`{ type = null, name = "n" }`), ExpectError: regexp.MustCompile(`must not be\s+null`)},
		},
	})
}

func TestAccSchemeFunctionErrors(t *testing.T) {
	cfg := func(args string) string {
		return `output "x" { value = provider::naming::scheme(` + args + `) }`
	}
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_8_0)},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg(`"{org}-{name", {}`), ExpectError: regexp.MustCompile(`unbalanced`)},
			{Config: cfg(`"{org}-{name}", { orgg = "acme" }`), ExpectError: regexp.MustCompile(`not\s+used\s+by\s+the\s+template`)},
			{Config: cfg(`"static", {}`), ExpectError: regexp.MustCompile(`no\s+placeholders`)},
		},
	})
}

const testAccKeyVault = `
locals {
  naming = provider::naming::scheme(
    "{type}-{system}-{name}-{environment}-{region}-{discriminator}",
    {},
    {
      type               = { max_length = 4 }
      system      = { max_length = 3 }
      name               = { on_too_long = "truncate" }
      environment = { allowed = ["dev", "test", "prod"] }
      region    = { max_length = 3 }
      discriminator      = { max_length = 4 }
    },
  )

  key_vault = provider::naming::rules(local.naming, {
    max_length = 24
    fit        = "name"
    components = { type = { max_length = 2 } }
  })

  parts = { type = "kv", system = "lis", region = "weu", discriminator = "b73a" }
}

output "dev"  { value = provider::naming::format(local.naming, merge(local.parts, { name = "wharf", environment = "dev" }), local.key_vault) }
output "prod" { value = provider::naming::format(local.naming, merge(local.parts, { name = "wharf", environment = "prod" }), local.key_vault) }
`

func TestAccKeyVault(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_8_0)},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccKeyVault,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("dev", "kv-lis-wha-dev-weu-b73a"),
					resource.TestCheckOutput("prod", "kv-lis-wha-prod-weu-b73a"),
				),
			},
		},
	})
}

func TestAccRulesAndComponentErrors(t *testing.T) {
	scheme := `provider::naming::scheme("{type}-{env}-{name}", {}, { env = { allowed = ["dev", "prod"] }, type = { max_length = 4 } })`
	cfg := func(body string) string {
		return `locals { naming = ` + scheme + ` }
` + body
	}
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_8_0)},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      cfg(`output "x" { value = provider::naming::format(local.naming, { type = "kv", env = "staging", name = "a" }) }`),
				ExpectError: regexp.MustCompile(`not\s+one\s+of\s+the\s+allowed`),
			},
			{
				Config:      cfg(`output "x" { value = provider::naming::rules(local.naming, { components = { type = { max_length = 9 } } }) }`),
				ExpectError: regexp.MustCompile(`looser`),
			},
			{
				Config:      cfg(`output "x" { value = provider::naming::rules(local.naming, { fit = "name", max_length = 8 }) }`),
				ExpectError: regexp.MustCompile(`no\s+room`),
			},
			{
				Config:      cfg(`output "x" { value = provider::naming::rules(local.naming, { lowercse = true }) }`),
				ExpectError: regexp.MustCompile(`unknown\s+field`),
			},
			{
				Config:      `output "x" { value = provider::naming::scheme("{a}", {}, { b = { max_length = 3 } }) }`,
				ExpectError: regexp.MustCompile(`not\s+used\s+by\s+the\s+template`),
			},
		},
	})
}
