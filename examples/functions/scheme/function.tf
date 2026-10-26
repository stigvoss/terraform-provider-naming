# A scheme is a template plus the values shared by everything that uses it.
locals {
  simple = provider::naming::scheme(
    "{org}-{env}-{region}-{type}-{name}",
    {
      org    = "acme"
      env    = "prod"
      region = "weu"
    },
  )
}

# Constraints per component are optional. A component without any is unlimited.
# Static values are checked against them when the scheme is defined.
#
#   min_length / max_length   length limits
#   allowed                   list of accepted values
#   pattern                   regular expression the value must match
#   overflow                  "error" (default) or "truncate"
#   hash                      hash characters appended when truncating
#
# Rules per resource type are the optional fourth argument. `by` names the
# placeholder whose value selects the rules; a value with no entry has none.
locals {
  naming = provider::naming::scheme(
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
    {
      by = "type"
      rules = {
        # Key Vault: 24 characters, hyphens allowed.
        #   24 - 5 hyphens - 2 (type) - 3 - 4 - 3 - 4 leaves 3 characters for `name`.
        kv = {
          max_length = 24
          fit        = "name"
          components = { type = { max_length = 2 } } # tightens the scheme's 4
        }

        # Storage account: 3-24 lowercase letters and digits, no hyphens.
        #   `remove` deletes every character that is not a lowercase letter or digit
        #   (the ^ means "not"), hyphens included, before the budget is calculated,
        #   so `name` gets 24 - 2 - 3 - 4 - 3 - 4 = 8 characters.
        st = {
          lowercase  = true
          remove     = "[^a-z0-9]"
          min_length = 3
          max_length = 24
          fit        = "name"
          components = { type = { max_length = 2 } }
        }
      }
    },
  )
}

# kv-lis-wha-dev-weu-b73a
output "key_vault_name" {
  value = provider::naming::format(local.naming, { type = "kv", name = "wharf" })
}

# stlisdatalakedevweub73a
output "storage_account_name" {
  value = provider::naming::format(local.naming, { type = "st", name = "datalake" })
}

# rg-lis-network-dev-weu-b73a (no rules for rg, so only the component constraints apply)
output "resource_group_name" {
  value = provider::naming::format(local.naming, { type = "rg", name = "network" })
}
