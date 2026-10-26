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
# Static values are checked against them when the scheme is defined.
#
#   min_length / max_length   length limits
#   allowed                   list of accepted values
#   pattern                   regular expression the value must match
#   overflow                  "error" (default) or "truncate"
#   hash                      hash characters appended when truncating
locals {
  constrained = {
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
}
