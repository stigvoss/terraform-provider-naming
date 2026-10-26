package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ function.Function = (*schemeFunction)(nil)

// NewSchemeFunction returns the provider::naming::scheme function.
func NewSchemeFunction() function.Function { return &schemeFunction{} }

type schemeFunction struct{}

// schemeAttrTypes is the shape shared by scheme's result and name's config parameter.
var schemeAttrTypes = map[string]attr.Type{
	"template": types.StringType,
	"static":   types.MapType{ElemType: types.StringType},
}

func (f *schemeFunction) Metadata(_ context.Context, _ function.MetadataRequest, resp *function.MetadataResponse) {
	resp.Name = "scheme"
}

func (f *schemeFunction) Definition(_ context.Context, _ function.DefinitionRequest, resp *function.DefinitionResponse) {
	resp.Definition = function.Definition{
		Summary:     "Validate a naming template and return a config for `name`",
		Description: "Checks the template syntax and that every `static` key is used by the template, then returns the config object to pass to `name`. Errors surface where the scheme is defined.",
		Parameters: []function.Parameter{
			function.StringParameter{
				Name:               "template",
				Description:        "Template with `{key}` placeholders, e.g. `{org}-{env}-{type}-{name}`.",
				AllowUnknownValues: true,
			},
			function.MapParameter{
				Name:               "static",
				Description:        "Module-wide values for placeholders (use {} for none).",
				ElementType:        types.StringType,
				AllowUnknownValues: true,
			},
		},
		Return: function.ObjectReturn{AttributeTypes: schemeAttrTypes},
	}
}

func (f *schemeFunction) Run(ctx context.Context, req function.RunRequest, resp *function.RunResponse) {
	var template types.String
	var static types.Map
	if resp.Error = req.Arguments.Get(ctx, &template, &static); resp.Error != nil {
		return
	}

	if template.IsNull() || static.IsNull() {
		resp.Error = function.NewArgumentFuncError(0, "template and static must not be null")
		return
	}

	// Validate whatever is already known; keys of a known map are known even
	// when its values are not.
	if !template.IsUnknown() {
		tpl, err := ParseTemplate(template.ValueString())
		if err != nil {
			resp.Error = function.NewArgumentFuncError(0, err.Error())
			return
		}
		if !static.IsUnknown() {
			keys := make([]string, 0, len(static.Elements()))
			for k := range static.Elements() {
				keys = append(keys, k)
			}
			if unused := tpl.UnusedKeys(keys); len(unused) > 0 {
				resp.Error = function.NewArgumentFuncError(1, fmt.Sprintf("static key(s) %s are not used by the template %q", quoteAll(unused), template.ValueString()))
				return
			}
		}
	}

	out, diags := types.ObjectValue(schemeAttrTypes, map[string]attr.Value{"template": template, "static": static})
	if diags.HasError() {
		resp.Error = function.FuncErrorFromDiags(ctx, diags)
		return
	}
	resp.Error = resp.Result.Set(ctx, out)
}
