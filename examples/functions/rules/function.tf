# The module-wide values (system, environment, region, discriminator) belong to
# the scheme. Each component also declares its largest possible length, which is
# what makes the budget for `name` predictable.
locals {
  scheme = provider::naming::scheme(
    "{type}-{system}-{name}-{environment}-{region}-{discriminator}",
    {
      system        = "lis"
      environment   = "dev"
      region        = "weu"
      discriminator = "b73a"
    },
    {
      type          = { max_length = 4 }
      system        = { max_length = 3 }
      name          = { overflow = "truncate" }
      environment   = { allowed = ["dev", "test", "prod"] }
      region        = { max_length = 3 }
      discriminator = { max_length = 4 }
    },
  )
}

# Everything callers need, in one place: local.naming.scheme and
# local.naming.rules.<resource type>. (A local cannot reference itself, so the
# scheme is built above. In a shared module these would be outputs.)
locals {
  naming = {
    scheme = local.scheme

    rules = {
      # Key Vault: 24 characters, hyphens allowed.
      #   24 - 5 hyphens - 2 (type) - 3 - 4 - 3 - 4 leaves 3 characters for `name`.
      key_vault = provider::naming::rules(local.scheme, {
        max_length = 24
        fit        = "name"
        components = { type = { max_length = 2 } } # tightens the scheme's 4
      })

      # Storage account: 3-24 lowercase letters and digits, no hyphens.
      #   `remove` deletes every character that is not a lowercase letter or digit
      #   (the ^ means "not"), hyphens included, before the budget is calculated,
      #   so `name` gets 24 - 2 - 3 - 4 - 3 - 4 = 8 characters.
      storage_account = provider::naming::rules(local.scheme, {
        lowercase  = true
        remove     = "[^a-z0-9]"
        min_length = 3
        max_length = 24
        fit        = "name"
        components = { type = { max_length = 2 } }
      })
    }
  }
}

# kv-lis-wha-dev-weu-b73a
output "key_vault_name" {
  value = provider::naming::format(
    local.naming.scheme,
    { type = "kv", name = "wharf" },
    local.naming.rules.key_vault,
  )
}

# stlisdatalakedevweub73a
output "storage_account_name" {
  value = provider::naming::format(
    local.naming.scheme,
    { type = "st", name = "datalake" },
    local.naming.rules.storage_account,
  )
}
