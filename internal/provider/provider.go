package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

var (
	_ provider.Provider              = (*namingProvider)(nil)
	_ provider.ProviderWithFunctions = (*namingProvider)(nil)
)

// New returns a provider factory for the given version.
//
// The provider has no configuration: provider-defined functions run without
// access to provider configuration, so the template is a function argument.
func New(version string) func() provider.Provider {
	return func() provider.Provider { return &namingProvider{version: version} }
}

type namingProvider struct {
	version string
}

func (p *namingProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "naming"
	resp.Version = p.version
}

func (p *namingProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Names resources from one central template. Provides the `format` function only; there is nothing to configure.",
	}
}

func (p *namingProvider) Configure(context.Context, provider.ConfigureRequest, *provider.ConfigureResponse) {
}

func (p *namingProvider) Functions(context.Context) []func() function.Function {
	return []func() function.Function{NewSchemeFunction, NewFormatFunction}
}

func (p *namingProvider) Resources(context.Context) []func() resource.Resource { return nil }

func (p *namingProvider) DataSources(context.Context) []func() datasource.DataSource { return nil }
