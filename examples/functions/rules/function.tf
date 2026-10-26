# One scheme for every resource type. Each component declares its largest
# possible length, which is what makes the budget for `name` predictable.
locals {
  naming = provider::naming::scheme(
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

  parts = {
    "system-code"      = "lis"
    "environment-code" = "dev"
    "location-code"    = "weu"
    discriminator      = "b73a"
  }
}

# Key Vault: 24 characters, hyphens allowed.
#   24 - 5 hyphens - 2 (type) - 3 - 4 - 3 - 4 leaves 3 characters for `name`.
locals {
  key_vault = provider::naming::rules(local.naming, {
    max_length = 24
    fit        = "name"
    components = { type = { max_length = 2 } } # tightens the scheme's 4
  })
}

# kv-lis-wha-dev-weu-b73a
output "key_vault_name" {
  value = provider::naming::format(
    local.naming,
    merge(local.parts, { type = "kv", name = "wharf" }),
    local.key_vault,
  )
}

# Storage account: 3-24 lowercase letters and digits, no hyphens.
#   `remove` strips the template's hyphens before the budget is calculated, so
#   `name` gets 24 - 2 - 3 - 4 - 3 - 4 = 8 characters.
locals {
  storage_account = provider::naming::rules(local.naming, {
    lowercase  = true
    remove     = "[^a-z0-9]"
    min_length = 3
    max_length = 24
    fit        = "name"
    components = { type = { max_length = 2 } }
  })
}

# stlisdatalakedevweub73a
output "storage_account_name" {
  value = provider::naming::format(
    local.naming,
    merge(local.parts, { type = "st", name = "datalake" }),
    local.storage_account,
  )
}
