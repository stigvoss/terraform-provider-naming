# terraform-provider-naming

A Terraform provider whose only job is to name resources from one central template.
It has no resources and no data sources, only two [provider-defined functions](https://developer.hashicorp.com/terraform/language/functions#provider-defined-functions).
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

- `scheme(template, static, components?, rules?)` defines everything about a naming scheme: the template, the shared values, optional per-component constraints and optional per-resource-type rules. It validates all of it, so mistakes show up where the scheme is defined.
- `with(scheme, values)` and `override(scheme, values)` derive a new scheme with extra or replaced static values, keeping the constraints and rules (see [Several environments](#several-environments-with-and-override)).
- `format(scheme, variables)` fills the template with the static values plus the variable parts, and errors on a missing placeholder, an extra key, a call key that overrides a static one, or a constraint violation.

Placeholder names are arbitrary. `type` and `name` in these examples are not special: a template can use any names, such as `{site}-{slice}-{role}-{zone}`. The only place a name matters is `by` in the resource type rules, which can name any placeholder.

## Constraints

Resource types have different naming rules. A storage account allows only lowercase letters and digits and 3-24 characters; a Key Vault allows 24 characters including hyphens. Everything below is optional.

There are two places to set constraints, both in `scheme`:

| Where | Applies to | Argument |
|---|---|---|
| **Component constraints** | one placeholder, for every resource type | the third argument |
| **Resource type rules** | the whole name, for one resource type | the fourth argument |

### Function arguments

| Function | Required | Optional |
|---|---|---|
| `scheme` | `template`, `static` (`{}` for none) | `components`, `rules` (pass `{}` for `components` to set only `rules`) |
| `with` | the scheme, `values` | |
| `override` | the scheme, `values` | |
| `format` | the scheme, `variables` | |

Every field inside `components` and `rules` is optional. The **Needs** columns below list what a field depends on. An unknown field name is an error, so a typo is caught rather than ignored.

### Component constraints

Keyed by placeholder name. A component with no constraints (`{}`) is unlimited. Static values are checked when the scheme is defined, variables on every `format` call.

| Field | Type | Required | Needs | Description |
|---|---|---|---|---|
| `min_length` | number | optional | | Shortest accepted value (counted in characters). Shorter values are rejected. Not more than `max_length`. |
| `max_length` | number | optional | | Longest accepted value. Longer values are rejected, or cut to this length if `overflow = "truncate"`. It is also the component's declared maximum when a `fit` budget is calculated. |
| `allowed` | list of strings | optional | | The only accepted values, e.g. `["dev", "test", "prod"]`. Anything else is rejected. The longest entry is the component's declared maximum length. |
| `pattern` | string | optional | | Regular expression the whole value must match, e.g. `"^[a-z]+$"`. Values that don't match are rejected. |
| `overflow` | string | optional | a limit: `max_length`, or being the `fit` component | What happens to a value longer than its limit, either `max_length` or the room `fit` leaves. `"error"` (default) rejects it; `"truncate"` cuts it to the limit. With no limit it has no effect. |
| `hash` | number | optional | `overflow = "truncate"` | How many hex characters (1 to 64) of a hash of the original value to put at the end of the cut value, so two long values that start the same don't become identical (`hash = 2` gives e.g. `abcd1f` and `abcd9c`, not `abcd` twice). The hash is taken from this component's value only, so it is the same in every environment. Only applied when a value is actually cut. |

### Resource type rules

An object with two fields:

| Field | Type | Required | Description |
|---|---|---|---|
| `by` | string | required | The placeholder whose value selects the rules, usually the resource type abbreviation (`type`). It must be in the template. |
| `rules` | map | optional | Rules per value of `by`, for example `kv` and `st`. A value with no entry has no extra rules, so only the types that need them have to be listed. |

Each entry applies to one resource type's whole name, in the order below:

| Step | Field | Type | Required | Needs | Description |
|---|---|---|---|---|---|
| 1 | `lowercase` | bool | optional | | `true` lowercases every part of the name. For resources that only accept lowercase, such as storage accounts. |
| 2 | `remove` | string | optional | | Regular expression. Every match is deleted from the name, including the template's own text. `"-"` deletes the hyphens. `"[^a-z0-9]"` deletes everything that is not a lowercase letter or digit (the `^` inside the brackets means "not"). It should match single characters. |
| 3 | `fit` | string | optional | `max_length`, and a placeholder in the template | Names the component that gets the room left under `max_length` (see below). |
| 4 | `min_length`, `max_length` | number | optional | | Length limits for the finished name. A name outside them is an error. |
| | `components` | map | optional | placeholders in the template | Per-component `min_length`, `max_length` and `allowed` for this resource type. They can only make the scheme's constraints stricter, never looser: the scheme says `region` is up to 3 characters, a rule could say 2. |

Inside a rule, the `by` component is always the rule's key, so `type` is `kv` in the Key Vault rule and counts as 2 characters in its length budget, not the scheme's "up to 4". You don't need to repeat that in `components`. The keys of `rules` are checked too: each must be a valid value for the `by` component, so a key that is longer than the scheme allows for it is rejected. Put `allowed` on that component if you want a typo such as `kvv` rejected, because a value with no entry is not an error.

### Fitting a component into a length limit

`fit = "name"` makes `name` the component that gets whatever room is left under `max_length`. The room is calculated from the **declared** maximum lengths of the other components, not from the values in a given call. So `name` is trimmed the same way in `dev` and `prod`.

Key Vault, 24 characters:

| Part | Characters |
|---|---|
| `type` (always `kv` in this rule) | 2 |
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

- If the other components leave no room at all, `scheme` reports it when the scheme is defined.

### Example

A scheme with component constraints and rules for a Key Vault and a storage account. For the storage account, `remove` deletes every character that is not a lowercase letter or digit, hyphens included, before the budget is calculated, so `name` gets 8 characters.

```hcl
locals {
  naming = provider::naming::scheme(
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
    {
      by = "type"
      rules = {
        kv = {
          max_length = 24
          fit        = "name"
        }
        st = {
          lowercase  = true
          remove     = "[^a-z0-9]"
          min_length = 3
          max_length = 24
          fit        = "name"
        }
      }
    },
  )
}

# kv-lis-wha-dev-weu-b73a
provider::naming::format(local.naming, { type = "kv", name = "wharf" })

# stlisdatalakedevweub73a
provider::naming::format(local.naming, { type = "st", name = "datalake" })

# rg-lis-network-dev-weu-b73a (no rules for rg, so only the component constraints apply)
provider::naming::format(local.naming, { type = "rg", name = "network" })
```

## Recipes

### Several environments with `with` and `override`

When schemes differ only in a few static values, such as the region and discriminator of each environment, define the scheme once and derive the rest. The constraints and rules are kept, and the new values are checked against them.

```hcl
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

  naming = {
    own    = provider::naming::with(local.base, { region = "weu", discriminator = "b73a" })
    shared = provider::naming::with(local.base, { region = "neu", discriminator = "c11d" })
  }
}

# rg-wha-net-dev-weu-b73a
provider::naming::format(local.naming.own, { type = "rg", name = "net" })
```

- `with` only **adds**: a key that is already static is an error, so a stale value cannot silently win.
- `override` only **replaces**: `provider::naming::override(local.naming.own, { env = "prod" })` changes `env`, and fails if `env` is not already static.
- Both fail for a key that is not a placeholder of the template, and check new values against the component constraints.
- Values that are not known until apply are accepted and checked once they are known.

### Sharing constraints between schemes

`components` and `rules` are ordinary arguments, so a Terraform local can be shared. Schemes with the same placeholders can use the same local:

```hcl
locals {
  components = {
    type = { max_length = 4 }
    env  = { allowed = ["dev", "test", "prod"] }
  }

  own    = provider::naming::scheme("{type}-{name}-{env}", { env = "dev" }, local.components)
  shared = provider::naming::scheme("{type}-{name}-{env}", { env = "prod" }, local.components)
}
```

A constraint for a placeholder that a template does not have is an error (`component "site" is not used by the template`). This is deliberate, because it catches a misspelled component name. To share one larger set between schemes with different placeholders, keep the keys each template uses:

```hcl
locals {
  all_components = {
    type = { max_length = 4 }
    site = { pattern = "^[a-z]{3}$" }
    role = { allowed = ["fleet", "convey", "lift"] }
  }

  subscription_keys = ["site", "role"]

  subscription = provider::naming::scheme(
    "{site}-{role}",
    {},
    { for k, v in local.all_components : k => v if contains(local.subscription_keys, k) },
  )
}
```

Or compose a set from named parts with `merge(local.common, local.subscription_parts)`.

### A scheme without a type prefix

Any template works. This one names Service Bus subscriptions, where `site` is a three-letter code, `role` is one of a fixed set, and `zone` is a zero-padded warehouse zone or several joined by `-`:

```hcl
locals {
  subscription = provider::naming::scheme(
    "{site}-{slice}-{role}-{zone}",
    { slice = "wha" },
    {
      site = { pattern = "^[a-z]{3}$" }
      role = { allowed = ["fleet", "convey", "lift"] }
      zone = { pattern = "^[0-9]{2}(-[0-9]{2})*$" }
    },
  )
}

# las-wha-fleet-04
provider::naming::format(local.subscription, { site = "las", role = "fleet", zone = "04" })

# las-wha-lift-04-06
provider::naming::format(local.subscription, { site = "las", role = "lift", zone = "04-06" })

# LAS-WHA-FLEET-04, for example a subject filter that must match the name in upper case
upper(provider::naming::format(local.subscription, { site = "las", role = "fleet", zone = "04" }))
```

For a variant of a name, such as the upper-cased filter, use Terraform's own `upper()` and `lower()`. `format` returns one string.

### Using a name

The result is an ordinary string, so it works anywhere a name is accepted, for a data source or a module input:

```hcl
data "azurerm_resource_group" "shared" {
  name = provider::naming::format(local.naming.shared, { type = "rg", name = "net" })
}

module "storage" {
  source = "./modules/storage"
  name   = provider::naming::format(local.naming.own, { type = "st", name = "data" })
}
```

Provider functions cannot read provider configuration, so the template is passed as an argument rather than set in a `provider` block. Share one scheme across configurations with a module output.

See [docs/](docs/) for the full function reference.

## Development

```sh
make test        # unit tests and tests that run Terraform (needs a terraform binary, 1.8+)
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
