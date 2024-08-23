// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"bytes"
	"context"
	"text/template"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ function.Function = FormatFunction{}
)

func NewFormatFunction() function.Function {
	return FormatFunction{}
}

type FormatFunction struct{}

func (r FormatFunction) Metadata(_ context.Context, req function.MetadataRequest, resp *function.MetadataResponse) {
	resp.Name = "format"
}

func (r FormatFunction) Definition(_ context.Context, _ function.DefinitionRequest, resp *function.DefinitionResponse) {
	resp.Definition = function.Definition{
		Summary:             "Format function",
		MarkdownDescription: "Formats a resource name according to defined standard.",
		Parameters: []function.Parameter{
			function.StringParameter{
				Name:                "resource",
				MarkdownDescription: "Resource type abbreviation.",
			},
			function.StringParameter{
				Name:                "resource_name",
				MarkdownDescription: "Resource name.",
			},
			function.ObjectParameter{
				Name:                "config",
				MarkdownDescription: "",
				AttributeTypes: map[string]attr.Type{
					"template": types.StringType,
					"args": types.MapType{
						ElemType: types.StringType,
					},
				},
			},
		},
		Return: function.StringReturn{},
	}
}

func (r FormatFunction) Run(ctx context.Context, req function.RunRequest, resp *function.RunResponse) {
	var resource string
	var resourceName string
	var config struct {
		Template string            `tfsdk:"template"`
		Args     map[string]string `tfsdk:"args"`
	}

	resp.Error = function.ConcatFuncErrors(req.Arguments.Get(ctx, &resource, &resourceName, &config))

	if resp.Error != nil {
		return
	}

	tmpl, err := template.New("template").Parse(config.Template)

	if err != nil {
		return
	}

	var result bytes.Buffer
	err = tmpl.Execute(&result, config.Args)

	if err != nil {
		return
	}

	name := result.String()

	resp.Error = function.ConcatFuncErrors(resp.Result.Set(ctx, name))
}
