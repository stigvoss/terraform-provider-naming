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
  naming = provider::naming::scheme(
    "{org}-{env}-{region}-{type}-{name}",
    { org = "acme", env = "prod", region = "weu" },
  )
}

resource "azurerm_resource_group" "rg" {
  name = provider::naming::format(local.naming, { type = "rg", name = "network" })
  # -> acme-prod-weu-rg-network
}
```

- `scheme(template, static)` validates the template and returns the config object. Mistakes show up where the scheme is defined.
- `format(config, variables)` fills the template with the static values plus the variable parts, and errors on a missing placeholder, an extra key, or a call key that overrides a static one.

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
