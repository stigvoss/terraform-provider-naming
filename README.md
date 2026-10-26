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

Resource types have different naming rules. A storage account allows only lowercase letters and digits and 3-24 characters; a Key Vault allows 24 characters including hyphens. Everything below is optional.

There are two places to set constraints:

| Where | Applies to | Set with |
|---|---|---|
| **Component constraints** | one placeholder, for every resource type | the third argument of `scheme` |
| **Resource type rules** | the whole name, for one resource type | `rules`, passed as the third argument of `format` |

### Component constraints

Keyed by placeholder name, in `scheme`. A component with no constraints is unlimited. Static values are checked when the scheme is defined, variables on every `format` call.

| Field | Meaning |
|---|---|
| `min_length`, `max_length` | length limits |
| `allowed` | list of accepted values |
| `pattern` | regular expression the value must match |
| `overflow` | what to do with a value over its limit: `"error"` (default) or `"truncate"` |
| `hash` | number of hash characters appended when truncating, so two long values don't collapse into one |

### Resource type rules

In `rules`. These apply to the finished name, in this order:

| Step | Field | Meaning |
|---|---|---|
| 1 | `lowercase` | lowercase every part |
| 2 | `remove` | regular expression of characters to delete (should match single characters) |
| 3 | `fit` | the component that is shrunk so the name fits `max_length` (see below) |
| 4 | `min_length`, `max_length` | length limits for the whole name |
| | `components` | per-component constraints that **tighten** the scheme's for this type, never loosen them |

### Fitting a component into a length limit

`fit = "name"` makes `name` the component that gets whatever room is left under `max_length`. The room is calculated from the **declared** maximum lengths of the other components, not from the values in a given call. So `name` is trimmed the same way in `dev` and `prod`.

Key Vault, 24 characters:

| Part | Characters |
|---|---|
| `type` (tightened from 4 to 2 for this type) | 2 |
| `system` | 3 |
| `environment` (longest allowed value) | 4 |
| `region` | 3 |
| `discriminator` | 4 |
| 5 hyphens | 5 |
| **Left for `name`** | **3** |

Things to know:

- A component without a declared maximum counts at its actual length. Declare one wherever the name must stay the same across environments.
- `fit` only sets the limit. Whether an over-long `name` is trimmed or rejected is the component's `overflow`:

| `overflow` of the fitted component | `name = "wharf"`, limit 3 |
|---|---|
| `"truncate"` | `wha` |
| `"error"` (default) | error: `"wharf"` is 5 characters, the limit is 3 |

- If the other components leave no room at all, `rules` reports it up front.

### Example

A scheme with constraints, and rules for a Key Vault and a storage account. `remove` strips the template's hyphens before the budget is calculated, so for the storage account `name` gets 8 characters.

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
      name          = { overflow = "truncate" }
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
