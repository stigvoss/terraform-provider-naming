package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ function.Function = (*formatFunction)(nil)

// NewFormatFunction returns the provider::naming::format function.
func NewFormatFunction() function.Function { return &formatFunction{} }

type formatFunction struct{}

func (f *formatFunction) Metadata(_ context.Context, _ function.MetadataRequest, resp *function.MetadataResponse) {
	resp.Name = "format"
}

func (f *formatFunction) Definition(_ context.Context, _ function.DefinitionRequest, resp *function.DefinitionResponse) {
	resp.Definition = function.Definition{
		Summary: "Render a resource name from a naming scheme",
		Description: "Fills the scheme's template with its static values plus the variable parts in `variables`, " +
			"checking each component against the scheme's constraints. Define the scheme once with `scheme` and pass it to every call.\n\n" +
			"If the scheme has resource type rules, the rules for the value of its `by` component (for example `type`) are applied " +
			"in this order:\n\n" +
			"1. `lowercase`: lowercase every part of the name.\n" +
			"2. `remove`: delete every character matching a regular expression, template text included.\n" +
			"3. `fit`: give one component the room left under `max_length`. The room is calculated from the declared " +
			"maximum lengths of the other components, so it is the same in every environment.\n" +
			"4. `min_length`, `max_length`: check the length of the finished name.",
		Parameters: []function.Parameter{
			function.ObjectParameter{
				Name:               "config",
				Description:        "A scheme, as returned by `scheme`, including any resource type rules.",
				AttributeTypes:     schemeAttrTypes,
				AllowUnknownValues: true,
			},
			function.MapParameter{
				Name:               "variables",
				Description:        "Values for the placeholders not set in the scheme's `static`.",
				ElementType:        types.StringType,
				AllowUnknownValues: true,
			},
		},
		Return: function.StringReturn{},
	}
}

func (f *formatFunction) Run(ctx context.Context, req function.RunRequest, resp *function.RunResponse) {
	var obj types.Object
	var vars types.Map
	if resp.Error = req.Arguments.Get(ctx, &obj, &vars); resp.Error != nil {
		return
	}

	if hasUnknown(obj) || hasUnknown(vars) {
		resp.Error = resp.Result.Set(ctx, types.StringUnknown())
		return
	}
	if obj.IsNull() {
		resp.Error = function.NewArgumentFuncError(0, "config must not be null")
		return
	}
	if vars.IsNull() {
		resp.Error = function.NewArgumentFuncError(1, "variables must not be null")
		return
	}

	cfg := obj.Attributes()
	template, _ := cfg["template"].(types.String)
	static, _ := cfg["static"].(types.Map)
	if template.IsNull() || static.IsNull() {
		resp.Error = function.NewArgumentFuncError(0, "config.template and config.static must not be null")
		return
	}
	staticVals, ferr := stringMap(ctx, static, 0, "config.static")
	if ferr != nil {
		resp.Error = ferr
		return
	}
	call, ferr := stringMap(ctx, vars, 1, "variables")
	if ferr != nil {
		resp.Error = ferr
		return
	}
	comps, err := ParseComponents(cfg["components"])
	if err != nil {
		resp.Error = function.NewArgumentFuncError(0, "config."+err.Error())
		return
	}
	ruleset, err := ParseRuleset(cfg["rules"])
	if err != nil {
		resp.Error = function.NewArgumentFuncError(0, "config."+err.Error())
		return
	}

	tpl, err := ParseTemplate(template.ValueString())
	if err != nil {
		resp.Error = function.NewArgumentFuncError(0, err.Error())
		return
	}
	// The selecting component's value comes from the call, or from the static
	// values when the scheme fixes it. A value with no entry has no extra rules.
	var rules Rules
	if ruleset.By != "" {
		selector, ok := call[ruleset.By]
		if !ok {
			selector = staticVals[ruleset.By]
		}
		rules = ruleset.For(selector)
	}

	name, err := tpl.Format(staticVals, call, comps, rules)
	if err != nil {
		resp.Error = function.NewArgumentFuncError(argOf(err, 1), withScheme(err, template.ValueString()))
		return
	}
	resp.Error = resp.Result.Set(ctx, name)
}
