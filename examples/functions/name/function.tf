terraform {
  required_providers {
    naming = {
      source = "stigvoss/naming"
    }
  }
}

# Validated once, where the scheme is defined (or put it in a shared module output).
locals {
  naming = provider::naming::scheme(
    "{org}-{env}-{region}-{type}-{name}",
    { org = "acme", env = "prod", region = "weu" },
  )
}

output shared across configurations).
locals {
  naming = {
    template = "{org}-{env}-{region}-{type}-{name}"
    static = {
      org    = "acme"
      env    = "prod"
      region = "weu"
    }
  }
}

output "resource_group_name" {
  # acme-prod-weu-rg-network
  value = provider::naming::name(local.naming, { type = "rg", name = "network" })
}
