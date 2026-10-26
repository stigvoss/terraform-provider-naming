locals {
  naming = provider::naming::scheme(
    "{org}-{env}-{region}-{type}-{name}",
    {
      org    = "acme"
      env    = "prod"
      region = "weu"
    },
  )
}

# acme-prod-weu-rg-network
output "resource_group_name" {
  value = provider::naming::format(local.naming, {
    type = "rg"
    name = "network"
  })
}

# Pass rules as a third argument for resource types with their own limits.
# See the `rules` function for a complete Key Vault and storage account example.
