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
  value = provider::naming::name(local.naming, { type = "rg", name = "network" })
}
`

func TestAccNameFunction(t *testing.T) {
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

func TestAccNameFunctionErrors(t *testing.T) {
	cfg := func(call string) string {
		return `
locals {
  naming = {
    template = "{org}-{type}-{name}"
    static   = { org = "acme" }
  }
}
output "x" { value = provider::naming::name(local.naming, ` + call + `) }
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
