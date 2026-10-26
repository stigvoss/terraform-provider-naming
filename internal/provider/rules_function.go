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
		Description: "Returns the rules, ready to pass to `format`, after checking them against the scheme, so mistakes surface where " +
			"the rules are defined instead of on the first `format` call.\n\n" +
			"Checks:\n\n" +
			"- the rules are well formed and use only known fields;\n" +
			"- `components` overrides only tighten the scheme's constraints;\n" +
			"- the `fit` component has room under `max_length`.\n\n" +
			"| Field | Description |\n" +
			"|---|---|\n" +
			"| `lowercase` | `true` lowercases every part of the name, for resources that only accept lowercase. |\n" +
			"| `remove` | Regular expression. Every match is deleted from the name, including the template's own text. " +
			"`\"-\"` deletes the hyphens; `\"[^a-z0-9]\"` deletes everything that is not a lowercase letter or digit " +
			"(the `^` inside the brackets means \"not\"). It should match single characters. |\n" +
			"| `min_length`, `max_length` | Length limits for the finished name. A name outside them is an error. |\n" +
			"| `fit` | The component that gets the room left under `max_length`. Needs `max_length`. |\n" +
			"| `components` | Per-component `min_length`, `max_length` and `allowed` for this resource type. " +
			"They can only be stricter than the scheme's, never looser. |",
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
