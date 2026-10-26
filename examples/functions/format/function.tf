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

# Constraints and per-resource-type rules are part of the scheme.
# See the `scheme` function for a complete Key Vault and storage account example.
