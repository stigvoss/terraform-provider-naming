package provider

import (
	"context"
	"fmt"

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
			"The optional third argument holds per-resource-type rules: `lowercase` (bool), `remove` (regular expression of characters to delete), " +
			"`min_length` and `max_length` (of the whole name), `fit` (the component that is shrunk to make the name fit `max_length`) and " +
			"`components` (constraints that tighten, never loosen, the scheme's). The budget for the `fit` component is calculated from the " +
			"declared bounds of the other components, so it is the same in every environment.",
		Parameters: []function.Parameter{
			function.ObjectParameter{
				Name:               "config",
				Description:        "A scheme, as returned by `scheme`.",
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
		VariadicParameter: function.DynamicParameter{
			Name:               "rules",
			Description:        "Optional per-resource-type rules.",
			AllowUnknownValues: true,
			AllowNullValue:     true,
		},
		Return: function.StringReturn{},
	}
}

func (f *formatFunction) Run(ctx context.Context, req function.RunRequest, resp *function.RunResponse) {
	var obj types.Object
	var vars types.Map
	var rest []types.Dynamic
	if resp.Error = req.Arguments.Get(ctx, &obj, &vars, &rest); resp.Error != nil {
		return
	}
	if len(rest) > 1 {
		resp.Error = function.NewArgumentFuncError(2, "format takes at most one rules argument")
		return
	}

	unknown := obj.IsUnknown() || vars.IsUnknown() || (len(rest) == 1 && hasUnknown(rest[0]))
	if !unknown && (hasUnknown(obj) || hasUnknown(vars)) {
		unknown = true
	}
	if unknown {
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
	var rules Rules
	if len(rest) == 1 {
		if rules, err = ParseRules(rest[0]); err != nil {
			resp.Error = function.NewArgumentFuncError(2, err.Error())
			return
		}
	}

	tpl, err := ParseTemplate(template.ValueString())
	if err != nil {
		resp.Error = function.NewArgumentFuncError(0, err.Error())
		return
	}
	staticKeys := make([]string, 0, len(staticVals))
	for k := range staticVals {
		staticKeys = append(staticKeys, k)
	}
	if unused := tpl.UnusedKeys(staticKeys); len(unused) > 0 {
		resp.Error = function.NewArgumentFuncError(0, fmt.Sprintf("config.static key(s) %s are not used by the template %q", quoteAll(unused), template.ValueString()))
		return
	}

	name, err := tpl.Format(staticVals, call, comps, rules)
	if err != nil {
		resp.Error = function.NewArgumentFuncError(1, err.Error())
		return
	}
	resp.Error = resp.Result.Set(ctx, name)
}
