locals {
  naming = provider::naming::scheme(
    "{org}-{env}-{region}-{type}-{name}",
    { org = "acme", env = "prod", region = "weu" },
  )
}

# acme-prod-weu-rg-network
output "resource_group_name" {
  value = provider::naming::format(local.naming, { type = "rg", name = "network" })
}
