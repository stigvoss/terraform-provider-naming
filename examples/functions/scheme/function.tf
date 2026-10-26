# A scheme is a template plus the values shared by everything that uses it.
locals {
  naming = provider::naming::scheme(
    "{org}-{env}-{region}-{type}-{name}",
    {
      org    = "acme"
      env    = "prod"
      region = "weu"
    },
  )
}

# Constraints per component are optional. A component without any is unlimited.
#
#   min_length / max_length   length limits
#   allowed                   list of accepted values
#   pattern                   regular expression the value must match
#   on_too_long               "error" (default) or "truncate"
#   hash                      hash characters appended when truncating
locals {
  constrained = provider::naming::scheme(
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
}
