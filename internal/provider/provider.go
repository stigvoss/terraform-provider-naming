// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// Ensure ScaffoldingProvider satisfies various provider interfaces.
var _ provider.Provider = &NamingProvider{}
var _ provider.ProviderWithFunctions = &NamingProvider{}

// NamingProvider defines the provider implementation.
type NamingProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

func (p *NamingProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "naming"
	resp.Version = p.version
}

func (p *NamingProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{},
	}
}

func (p *NamingProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	if resp.Diagnostics.HasError() {
		return
	}
}

func (p *NamingProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{}
}

func (p *NamingProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}

func (p *NamingProvider) Functions(ctx context.Context) []func() function.Function {
	return []func() function.Function{
		NewFormatFunction,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &NamingProvider{
			version: version,
		}
	}
}
