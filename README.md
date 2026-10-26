# terraform-provider-naming

A Terraform provider whose only job is to name resources from one central template.
It has no resources and no data sources, only three [provider-defined functions](https://developer.hashicorp.com/terraform/language/functions#provider-defined-functions).
Requires Terraform **1.8+**.

```hcl
terraform {
  required_providers {
    naming = { source = "stigvoss/naming" }
  }
}

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

resource "azurerm_resource_group" "rg" {
  name = provider::naming::format(local.naming.scheme, { type = "rg", name = "network" })
  # -> acme-prod-weu-rg-network
}
```

- `scheme(template, static, components?)` validates the template and optional per-component constraints, and returns the config object. Mistakes show up where the scheme is defined.
- `rules(scheme, rules)` validates per-resource-type rules against a scheme and returns them.
- `format(scheme, variables, rules?)` fills the template with the static values plus the variable parts, and errors on a missing placeholder, an extra key, a call key that overrides a static one, or a constraint violation.

## Constraints

Resource types have different naming rules (an Azure storage account allows only lowercase letters and digits and 3-24 characters; a Key Vault allows 24 characters with hyphens). There are two levels, and all of it is optional.

**Per component, in the scheme.** Each component can set `min_length`, `max_length`, `allowed` (a list of values), `pattern` (a regular expression), `on_too_long` (`"error"`, the default, or `"truncate"`) and `hash` (number of hash characters appended when truncating, so two long values do not collapse into one). A component with no constraints is unlimited.

**Per resource type, in the rules.** `lowercase`, `remove` (a regular expression of characters to delete), `min_length` / `max_length` for the whole name, `fit`, and `components`, which can only *tighten* the scheme's component constraints for that type (for example `type` is up to 4 characters in general but 2 for a Key Vault).

`fit` names the component that is shrunk to make the name fit `max_length`. Its budget is calculated from the *declared* maximum lengths of the other components, not their actual values, so the shrunk component is the same in every environment (`dev` and `prod` get the same trimmed `name`). A component with no declared maximum counts at its actual length, so declare one wherever the name has to stay constant. Whether `fit` trims or errors follows the component's `on_too_long`.

```hcl
locals {
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
      name          = { on_too_long = "truncate" }
      environment   = { allowed = ["dev", "test", "prod"] }
      region        = { max_length = 3 }
      discriminator = { max_length = 4 }
    },
  )
}

# A local cannot reference itself, so the scheme is built above and collected here.
# In a shared module, `scheme` and `rules` would be outputs.
locals {
  naming = {
    scheme = local.scheme

    rules = {
      key_vault = provider::naming::rules(local.scheme, {
        max_length = 24
        fit        = "name"
        components = { type = { max_length = 2 } }
      })

      # `remove` strips the template's hyphens before the budget is calculated,
      # so `name` gets 8 characters in every environment.
      storage_account = provider::naming::rules(local.scheme, {
        lowercase  = true
        remove     = "[^a-z0-9]"
        min_length = 3
        max_length = 24
        fit        = "name"
        components = { type = { max_length = 2 } }
      })
    }
  }
}

# kv-lis-wha-dev-weu-b73a  (24 - 5 hyphens - 2 - 3 - 4 - 3 - 4 leaves 3 characters for name)
provider::naming::format(
  local.naming.scheme,
  { type = "kv", name = "wharf" },
  local.naming.rules.key_vault,
)

# stlisdatalakedevweub73a
provider::naming::format(
  local.naming.scheme,
  { type = "st", name = "datalake" },
  local.naming.rules.storage_account,
)
```

Provider functions cannot read provider configuration, so the template is passed as an argument rather than set in a `provider` block. Share one scheme across configurations with a module output.

See [docs/](docs/) for the full function reference.

## Development

```sh
make test        # unit and acceptance tests (needs a terraform binary, 1.8+)
make generate    # regenerate docs with tfplugindocs
make lint
```

### Trying it locally

Use a [development override](https://developer.hashicorp.com/terraform/cli/config/config-file#development-overrides-for-provider-developers)
to run your own configurations against a local build, without publishing or running `terraform init`.

1. Build the provider into a folder:

   ```sh
   go build -o ~/terraform-dev/terraform-provider-naming .
   ```

2. Create `~/terraform-dev/dev.tfrc`:

   ```hcl
   provider_installation {
     dev_overrides { "stigvoss/naming" = "/Users/<you>/terraform-dev" }
     direct {}
   }
   ```

3. In a configuration that declares `naming` in `required_providers` (see [examples/](examples/)), point Terraform at that file:

   ```sh
   TF_CLI_CONFIG_FILE=~/terraform-dev/dev.tfrc terraform plan
   ```

Terraform warns that the provider is overridden; that is expected. Rebuild after every code change.
Setting `TF_CLI_CONFIG_FILE` per command leaves your `~/.terraformrc` untouched.

## Releasing

Pushing a `v*` tag runs [GoReleaser](.goreleaser.yml) and publishes signed release artifacts.
To publish on the [Terraform Registry](https://registry.terraform.io):

1. Make the repository public (the registry requires it, and the repo name must stay `terraform-provider-naming`).
2. Create a GPG key and add the private key and passphrase as the repository secrets `GPG_PRIVATE_KEY` and `PASSPHRASE`.
3. Add the public key under *User Settings → Signing Keys* on the registry, then publish the provider from the registry UI.
4. `git tag v0.1.0 && git push origin v0.1.0`.

## License

[MPL-2.0](LICENSE)
