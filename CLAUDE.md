# terraform-provider-naming

A Terraform provider whose only job is to name resources from one central template.
`scheme` validates a template plus module-wide static parts; `format` takes that scheme and
the variable parts and returns the rendered name.

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
  name = provider::naming::format(local.naming, { type = "rg", name = "network" })
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

### Function `scheme(template, static, components?)`
- Validates template syntax, that every `static` key is used, and the optional component
  constraints; returns `{ template, static, components }` for `format`.
- Component spec fields (all optional, unlimited by default): `min_length`, `max_length`,
  `allowed`, `pattern`, `overflow` (`error` default | `truncate`), `hash`.
- Unknown inputs yield an unknown result; whatever is known is still validated.

### Function `rules(scheme, rules)`
- Validates per-type rules against a scheme at definition time and returns them normalized.
- Fields: `lowercase`, `remove` (regex), `min_length`, `max_length`, `fit`, `components`
  (overrides may only tighten the scheme's `max_length` / `min_length` / `allowed`).

### Function `format(config, variables, rules?)`
- Pipeline (`format.go`): merge static + call values -> check each against its component spec
  -> `lowercase` / `remove` -> shrink the `fit` component into its budget -> render -> check total length.
- The `fit` budget uses the *declared* bound (`max_length` or longest `allowed`) of every other
  component, not actual lengths, so the fitted component is identical across environments. An
  unbounded component counts at its actual length.
- Errors: invalid template, unused `static` key, component or rule violation (arg 0/1/2); call key
  overriding `static`, extra call key, unresolved placeholder.
- Rendering is simple `{key}` replacement, no templating engine. Remove patterns should match
  single characters; they are applied to each component and each literal segment separately.

### Optional later
- Bundle the scheme into `rules()`'s result so a call is `format(local.naming.key_vault, { ... })`
  (two arguments instead of three). `format` would accept either a scheme or the bundle.
  Provider-level scheme config is not possible: functions cannot read provider configuration,
  and a data source would reintroduce "known after apply" names.
- Built-in presets of hard limits per cloud, as a pure function (e.g. `provider::naming::azurerm()`
  returning resource type -> base rules), merged with scheme-specific `fit` / `components`:
  `merge(local.azure.storage_account, { fit = "name", ... })`. Not a resource or data source: those
  would make names unknown at plan time. Needs a whole-name `pattern` rule first (Key Vault: starts
  with a letter, no consecutive hyphens). No type abbreviations (`kv`, `st`): that is org convention.

## Gotchas

- Provider functions run without provider configuration; handle unknown arguments.
- Each module that calls the function must declare the provider in `required_providers`.
- Function arguments and results appear in plans; do not echo secrets.
- Simpler alternative without a provider: a shared module with `locals` and `format()` / `templatestring()` (1.9+). The provider is worth it only for sharing one definition across many separate configurations and for stricter validation.

## Suggested layout

```
main.go
internal/provider/provider.go
internal/provider/format_function.go
internal/provider/render.go          # pure rendering logic, unit-tested
internal/provider/render_test.go
examples/
docs/
```

## First tasks

1. `go mod init` and scaffold from the terraform-provider-scaffolding-framework layout.
2. Implement `render.go` with table-driven unit tests (missing key, extra key, override, unbalanced braces).
3. Implement the provider and the `format` function.
4. Add acceptance tests using `terraform-plugin-testing` with a `provider::naming::format(...)` output (needs Terraform 1.8+).
5. Add an example under `examples/` and generate docs with `tfplugindocs`.
