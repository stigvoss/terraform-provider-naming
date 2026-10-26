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
}
