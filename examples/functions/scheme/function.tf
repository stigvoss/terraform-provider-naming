# A scheme is a template plus the values shared by everything that uses it.
locals {
  naming = {
    scheme = provider::naming::scheme(
      "{org}-{env}-{region}-{type}-{name}",
      {
        org    = "acme"
        env    = "prod"
        region = "weu"
      },
    )
  }
}

# Constraints per component are optional. A component without any is unlimited.
#
#   min_length / max_length   length limits
#   allowed                   list of accepted values
#   pattern                   regular expression the value must match
#   on_too_long               "error" (default) or "truncate"
#   hash                      hash characters appended when truncating
locals {
  constrained = {
    scheme = provider::naming::scheme(
      "{type}-{system}-{name}-{environment}-{location}-{discriminator}",
      {},
      {
        type          = { max_length = 4 }
        system        = { max_length = 3 }
        name          = { on_too_long = "truncate" }
        environment   = { allowed = ["dev", "test", "prod"] }
        location      = { max_length = 3 }
        discriminator = { max_length = 4 }
      },
    )
  }
}
