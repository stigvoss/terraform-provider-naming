// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

func TestExampleFunction_Known(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_8_0),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				output "test" {
					value = provider::naming::format("func", "orders", {
						template = "{{.resource}}-{{.system}}-{{.resourceName}}-{{.environment}}-{{.region}}-{{.discriminator}}",
						args = {
							"system" = "lis"
							"environment" = "dev"
							"region" = "weu"
							"discriminator" = "ugy4"
						}
					})
				}
				`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckOutput("test", "func-lis-orders-dev-weu-ugy4"),
				),
			},
		},
	})
}

// func TestExampleFunction_Null(t *testing.T) {
// 	resource.UnitTest(t, resource.TestCase{
// 		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
// 			tfversion.SkipBelow(tfversion.Version1_8_0),
// 		},
// 		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
// 		Steps: []resource.TestStep{
// 			{
// 				Config: `
// 				output "test" {
// 					value = provider::naming::format(null)
// 				}
// 				`,
// 				// The parameter does not enable AllowNullValue
// 				ExpectError: regexp.MustCompile(`argument must not be null`),
// 			},
// 		},
// 	})
// }
