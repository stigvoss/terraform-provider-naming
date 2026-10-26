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

# A Key Vault allows 24 characters. The `name` component gets a fixed budget calculated
# from the other components' declared maximum lengths, so it is trimmed identically in
# every environment: kv-lis-wha-dev-weu-b73a, kv-lis-wha-prod-weu-b73a
locals {
  kv_naming = provider::naming::scheme(
    "{type}-{system-code}-{name}-{environment-code}-{location-code}-{discriminator}",
    {},
    {
      type               = { max_length = 4 }
      "system-code"      = { max_length = 3 }
      name               = { on_too_long = "truncate" }
      "environment-code" = { allowed = ["dev", "test", "prod"] }
      "location-code"    = { max_length = 3 }
      discriminator      = { max_length = 4 }
    },
  )

  key_vault = provider::naming::rules(local.kv_naming, {
    max_length = 24
    fit        = "name"
    components = { type = { max_length = 2 } } # tightens the scheme's 4 for this type
  })
}

output "key_vault_name" {
  value = provider::naming::format(local.kv_naming, {
    type               = "kv"
    "system-code"      = "lis"
    name               = "wharf"
    "environment-code" = "dev"
    "location-code"    = "weu"
    discriminator      = "b73a"
  }, local.key_vault)
}
