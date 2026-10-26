# Constraints are optional; a component without any is unlimited.
locals {
  naming = provider::naming::scheme(
    "{type}-{system-code}-{name}-{environment-code}-{location-code}-{discriminator}",
    {},
    {
      type               = { max_length = 4 }
      "system-code"      = { max_length = 3 }
      name               = { on_too_long = "truncate" } # default is "error"
      "environment-code" = { allowed = ["dev", "test", "prod"] }
      "location-code"    = { max_length = 3 }
      discriminator      = { max_length = 4 }
    },
  )

  # The same scheme works for a storage account (3-24 lowercase letters and digits).
  # `remove` also strips the template's hyphens, leaving 24 - 2 - 3 - 4 - 3 - 4 = 8
  # characters for `name` in every environment.
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
  value = provider::naming::format(local.naming, {
    type               = "st"
    "system-code"      = "lis"
    name               = "datalake"
    "environment-code" = "dev"
    "location-code"    = "weu"
    discriminator      = "b73a"
  }, local.storage_account)
}
