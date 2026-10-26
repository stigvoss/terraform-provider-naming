package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ function.Function = (*rulesFunction)(nil)

// NewRulesFunction returns the provider::naming::rules function.
func NewRulesFunction() function.Function { return &rulesFunction{} }

type rulesFunction struct{}

func (f *rulesFunction) Metadata(_ context.Context, _ function.MetadataRequest, resp *function.MetadataResponse) {
	resp.Name = "rules"
}

func (f *rulesFunction) Definition(_ context.Context, _ function.DefinitionRequest, resp *function.DefinitionResponse) {
	resp.Definition = function.Definition{
		Summary: "Validate per-resource-type rules against a scheme",
		Description: "Checks that the rules are well formed, that component overrides only tighten the scheme's constraints, " +
			"and that the `fit` component has room. Returns the rules, ready to pass to `format`, so mistakes surface where " +
			"the rules are defined instead of on the first `format` call.",
		Parameters: []function.Parameter{
			function.ObjectParameter{
				Name:               "config",
				Description:        "A scheme, as returned by `scheme`.",
				AttributeTypes:     schemeAttrTypes,
				AllowUnknownValues: true,
			},
			function.DynamicParameter{
				Name:               "rules",
				Description:        "Rules: `lowercase`, `remove`, `min_length`, `max_length`, `fit` and `components`.",
				AllowUnknownValues: true,
			},
		},
		Return: function.ObjectReturn{AttributeTypes: rulesAttrTypes},
	}
}

func (f *rulesFunction) Run(ctx context.Context, req function.RunRequest, resp *function.RunResponse) {
	var obj types.Object
	var rulesArg types.Dynamic
	if resp.Error = req.Arguments.Get(ctx, &obj, &rulesArg); resp.Error != nil {
		return
	}
	if hasUnknown(obj) || hasUnknown(rulesArg) {
		resp.Error = resp.Result.Set(ctx, types.ObjectUnknown(rulesAttrTypes))
		return
	}
	if obj.IsNull() {
		resp.Error = function.NewArgumentFuncError(0, "config must not be null")
		return
	}
	cfg := obj.Attributes()
	template, _ := cfg["template"].(types.String)
	tpl, err := ParseTemplate(template.ValueString())
	if err != nil {
		resp.Error = function.NewArgumentFuncError(0, err.Error())
		return
	}
	comps, err := ParseComponents(cfg["components"])
	if err != nil {
		resp.Error = function.NewArgumentFuncError(0, "config."+err.Error())
		return
	}
	rules, err := ParseRules(rulesArg)
	if err != nil {
		resp.Error = function.NewArgumentFuncError(1, err.Error())
		return
	}
	if err := tpl.CheckRules(comps, rules); err != nil {
		resp.Error = function.NewArgumentFuncError(1, err.Error())
		return
	}
	resp.Error = resp.Result.Set(ctx, RulesValue(rules))
}
