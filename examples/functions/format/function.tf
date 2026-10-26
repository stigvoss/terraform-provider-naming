locals {
  naming = {
    scheme = provider::naming::scheme(
      "{org}-{env}-{region}-{type}-{name}",
      {
        org    = "acme"
        env    = "prod"
        region = "weu"
      },
    )
  }
}

# acme-prod-weu-rg-network
output "resource_group_name" {
  value = provider::naming::format(local.naming.scheme, {
    type = "rg"
    name = "network"
  })
}

# Pass rules as a third argument for resource types with their own limits,
# e.g. local.naming.rules.key_vault. See the `rules` function for a complete example.
