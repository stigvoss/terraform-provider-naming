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

func (f *schemeFunction) Metadata(_ context.Context, _ function.MetadataRequest, resp *function.MetadataResponse) {
	resp.Name = "scheme"
}

func (f *schemeFunction) Definition(_ context.Context, _ function.DefinitionRequest, resp *function.DefinitionResponse) {
	resp.Definition = function.Definition{
		Summary: "Validate a naming template and return a config for `format`",
		Description: "Checks the template syntax, that every `static` key is used by the template, and the optional per-component " +
			"constraints, then returns the config object to pass to `format`. Errors surface where the scheme is defined.\n\n" +
			"The optional third argument maps component names to constraints. Every field is optional, and a component without " +
			"constraints is unlimited.\n\n" +
			"| Field | Type | Required | Needs | Description |\n" +
			"|---|---|---|---|---|\n" +
			"| `min_length` | number | optional | | Shortest accepted value. Shorter values are rejected. |\n" +
			"| `max_length` | number | optional | | Longest accepted value. Longer values are rejected, or cut to this length if `overflow = \"truncate\"`. " +
			"Also the component's declared maximum when a `fit` budget is calculated. |\n" +
			"| `allowed` | list of strings | optional | | The only accepted values, e.g. `[\"dev\", \"test\", \"prod\"]`. The longest entry is the declared maximum length. |\n" +
			"| `pattern` | string | optional | | Regular expression the whole value must match, e.g. `\"^[a-z]+$\"`. |\n" +
			"| `overflow` | string | optional | a limit (`max_length`, or being the `fit` component) | What happens to a value longer than its limit: " +
			"`\"error\"` (default) rejects it, `\"truncate\"` cuts it to the limit. |\n" +
			"| `hash` | number | optional | `overflow = \"truncate\"` | Number of hex characters (1 to 64) of a hash of the original value appended to the cut value, " +
			"so long values that start the same stay distinct. Taken from this component's value only, so it is the same in every environment. |",
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
		VariadicParameter: function.DynamicParameter{
			Name:               "components",
			Description:        "Optional map of component name to constraints.",
			AllowUnknownValues: true,
			AllowNullValue:     true,
		},
		Return: function.ObjectReturn{AttributeTypes: schemeAttrTypes},
	}
}

func (f *schemeFunction) Run(ctx context.Context, req function.RunRequest, resp *function.RunResponse) {
	var template types.String
	var static types.Map
	var rest []types.Dynamic
	if resp.Error = req.Arguments.Get(ctx, &template, &static, &rest); resp.Error != nil {
		return
	}
	if len(rest) > 1 {
		resp.Error = function.NewArgumentFuncError(2, "scheme takes at most one components argument")
		return
	}
	if template.IsNull() || static.IsNull() {
		resp.Error = function.NewArgumentFuncError(0, "template and static must not be null")
		return
	}

	var compsVal attr.Value = types.MapNull(componentObjType)
	if len(rest) == 1 {
		compsVal = rest[0]
	}
	// Check whatever is already known: the template syntax, and the static keys
	// (known even when their values are not). Constraints on unknown values
	// wait until apply.
	var tpl *Template
	if !template.IsUnknown() {
		var err error
		if tpl, err = ParseTemplate(template.ValueString()); err != nil {
			resp.Error = function.NewArgumentFuncError(0, err.Error())
			return
		}
		if !static.IsUnknown() {
			if err := tpl.CheckStaticKeys(mapKeys(static)); err != nil {
				resp.Error = function.NewArgumentFuncError(1, err.Error())
				return
			}
		}
	}
	if template.IsUnknown() || hasUnknown(static) || hasUnknown(compsVal) {
		resp.Error = resp.Result.Set(ctx, types.ObjectUnknown(schemeAttrTypes))
		return
	}

	staticVals, ferr := stringMap(ctx, static, 1, "static")
	if ferr != nil {
		resp.Error = ferr
		return
	}

	comps, err := ParseComponents(compsVal)
	if err != nil {
		resp.Error = function.NewArgumentFuncError(2, err.Error())
		return
	}
	if err := tpl.CheckSpecs(comps); err != nil {
		resp.Error = function.NewArgumentFuncError(2, err.Error())
		return
	}
	for k, v := range staticVals {
		if _, err := CheckValue(k, v, comps[k]); err != nil {
			resp.Error = function.NewArgumentFuncError(1, "static "+err.Error())
			return
		}
	}

	out := types.ObjectValueMust(schemeAttrTypes, map[string]attr.Value{
		"template":   template,
		"static":     static,
		"components": ComponentsValue(comps),
	})
	resp.Error = resp.Result.Set(ctx, out)
}

func mapKeys(m types.Map) []string {
	keys := make([]string, 0, len(m.Elements()))
	for k := range m.Elements() {
		keys = append(keys, k)
	}
	return keys
}

// stringMap converts m to a Go map. Null elements are an error.
func stringMap(ctx context.Context, m types.Map, argPos int64, what string) (map[string]string, *function.FuncError) {
	var raw map[string]types.String
	if diags := m.ElementsAs(ctx, &raw, false); diags.HasError() {
		return nil, function.FuncErrorFromDiags(ctx, diags)
	}
	out := make(map[string]string, len(raw))
	for k, v := range raw {
		if v.IsNull() {
			return nil, function.NewArgumentFuncError(argPos, fmt.Sprintf("%s[%q] must not be null", what, k))
		}
		out[k] = v.ValueString()
	}
	return out, nil
}
