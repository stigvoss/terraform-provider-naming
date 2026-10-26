# terraform-provider-naming

A Terraform provider whose only job is to name resources from one central template.
The provider block holds the template and module-wide static parts. A provider-defined
function takes only the variable parts and returns the rendered name.

## Goal / usage

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
  name = provider::naming::name(local.naming, { type = "rg", name = "network" })
  # -> acme-prod-weu-rg-network
}
```

## Requirements

- Go, built on **terraform-plugin-framework** (SDKv2 does not support provider functions).
- Requires Terraform **1.8+** at runtime.
- No resources or data sources. Functions only.

## Design

**Provider functions cannot read provider configuration.** The framework has no
`FunctionData` / `function.ConfigureRequest`, so the original "template in the provider
block" idea is impossible. The provider has an empty schema, and the template travels as a
function argument. Share it across configurations via a module output.

### Function `scheme(template, static)`
- Validates template syntax and that every `static` key is used, then returns the
  `{ template, static }` object for `name`. Errors surface where the scheme is defined.
- Unknown inputs are passed through; whatever is known is still validated.

### Function `name(config, variables)`
- `config`: object `{ template = string, static = map(string) }` (`static = {}` for none).
- `variables`: map of strings for the remaining placeholders.
- Unknown anywhere in the arguments -> unknown result (plans do not fail).
- Errors: invalid template, unused `static` key (arg 0); call key overriding `static`,
  call key not in template, unresolved placeholder, null value (arg 1).
- Rendering is simple `{key}` replacement (`internal/provider/render.go`), no templating engine.

### Optional later
- Naming rules: max length, lowercase, allowed characters, separator handling (e.g. Azure storage accounts forbid hyphens).

## Gotchas

- Provider functions run without provider configuration; handle unknown arguments.
- Each module that calls the function must declare the provider in `required_providers`.
- Function arguments and results appear in plans; do not echo secrets.
- Simpler alternative without a provider: a shared module with `locals` and `format()` / `templatestring()` (1.9+). The provider is worth it only for sharing one definition across many separate configurations and for stricter validation.

## Suggested layout

```
main.go
internal/provider/provider.go
internal/provider/name_function.go
internal/provider/render.go          # pure rendering logic, unit-tested
internal/provider/render_test.go
examples/
docs/
```

## First tasks

1. `go mod init` and scaffold from the terraform-provider-scaffolding-framework layout.
2. Implement `render.go` with table-driven unit tests (missing key, extra key, override, unbalanced braces).
3. Implement the provider and the `name` function.
4. Add acceptance tests using `terraform-plugin-testing` with a `provider::naming::name(...)` output (needs Terraform 1.8+).
5. Add an example under `examples/` and generate docs with `tfplugindocs`.
