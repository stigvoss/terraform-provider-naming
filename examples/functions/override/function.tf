locals {
  dev = provider::naming::scheme(
    "{type}-{system}-{name}-{env}-{region}",
    {
      system = "wha"
      env    = "dev"
      region = "weu"
    },
    {
      env = { allowed = ["dev", "test", "prod"] }
    },
  )

  # `override` replaces a value that is already static. Use `with` to add a new one.
  prod = provider::naming::override(local.dev, { env = "prod" })
}

# rg-wha-net-prod-weu
output "prod_resource_group" {
  value = provider::naming::format(local.prod, { type = "rg", name = "net" })
}
