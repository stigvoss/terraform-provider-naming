locals {
  naming = provider::naming::scheme(
    "{org}-{env}-{region}-{type}-{name}",
    { org = "acme", region = "weu" },
    { env = { max_length = 4 }, type = { max_length = 4 } },
  )

  # Storage accounts: lowercase letters and digits only, 3-24 characters.
  storage_account = provider::naming::rules(local.naming, {
    lowercase  = true
    remove     = "[^a-z0-9]"
    min_length = 3
    max_length = 24
  })
}

# acmeprodweustdata
output "storage_account_name" {
  value = provider::naming::format(local.naming, { env = "prod", type = "st", name = "data" }, local.storage_account)
}
