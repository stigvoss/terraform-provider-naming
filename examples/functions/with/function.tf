# Define the scheme once, with the values shared by every environment.
locals {
  base = provider::naming::scheme(
    "{type}-{system}-{name}-{env}-{region}-{discriminator}",
    {
      system = "wha"
      env    = "dev"
    },
    {
      type          = { max_length = 4 }
      env           = { allowed = ["dev", "test", "prod"] }
      region        = { max_length = 3 }
      discriminator = { max_length = 4 }
    },
  )

  # Derive one scheme per environment. The constraints and rules are kept, and
  # the new values are checked against them.
  naming = {
    own    = provider::naming::with(local.base, { region = "weu", discriminator = "b73a" })
    shared = provider::naming::with(local.base, { region = "neu", discriminator = "c11d" })
  }
}

# rg-wha-net-dev-weu-b73a
output "own_resource_group" {
  value = provider::naming::format(local.naming.own, { type = "rg", name = "net" })
}

# rg-wha-net-dev-neu-c11d
output "shared_resource_group" {
  value = provider::naming::format(local.naming.shared, { type = "rg", name = "net" })
}
