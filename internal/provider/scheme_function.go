package provider

import (
	"context"

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
		Summary: "Define a naming scheme for `format`",
		Description: "Checks the template syntax, that every `static` key is used by the template, the optional per-component " +
			"constraints and the optional per-resource-type rules, then returns the scheme to pass to `format`. " +
			"Errors surface where the scheme is defined.\n\n" +
			"**Component constraints** (third argument) map component names to constraints. Every field is optional, and a " +
			"component without constraints is unlimited.\n\n" +
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
			"so long values that start the same stay distinct. Taken from this component's value only, so it is the same in every environment. |\n\n" +
			"**Resource type rules** (fourth argument) are an object with `by`, the placeholder whose value selects the rules " +
			"(for example `type`), and `rules`, a map from that value to the rules for it. A value with no entry has no extra rules. " +
			"Inside a rule the `by` component is always the rule's key, so a `kv` rule counts `type` as 2 characters in its length budget. " +
			"Every field in a rule is optional.\n\n" +
			"| Field | Type | Needs | Description |\n" +
			"|---|---|---|---|\n" +
			"| `lowercase` | bool | | `true` lowercases every part of the name, for resources that only accept lowercase. |\n" +
			"| `remove` | string | | Regular expression. Every match is deleted from the name, including the template's own text. " +
			"`\"-\"` deletes the hyphens; `\"[^a-z0-9]\"` deletes everything that is not a lowercase letter or digit " +
			"(the `^` inside the brackets means \"not\"). It should match single characters. |\n" +
			"| `min_length`, `max_length` | number | | Length limits for the finished name. A name outside them is an error. |\n" +
			"| `fit` | string | `max_length`, and a placeholder in the template | The component that gets the room left under `max_length`. |\n" +
			"| `components` | map | placeholders in the template | Per-component `min_length`, `max_length` and `allowed` for this resource type. " +
			"They can only be stricter than the scheme's, never looser. |",
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
			Name:               "options",
			Description:        "Optional: component constraints, then resource type rules (`{ by = ..., rules = { ... } }`). Pass `{}` for the first to set only the second.",
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
	if len(rest) > 2 {
		resp.Error = function.NewArgumentFuncError(4, "scheme takes at most a components argument and a rules argument")
		return
	}
	if template.IsNull() || static.IsNull() {
		resp.Error = function.NewArgumentFuncError(0, "template and static must not be null")
		return
	}

	var compsVal, rulesVal attr.Value = types.MapNull(componentObjType), types.ObjectNull(rulesetAttrTypes)
	if len(rest) > 0 {
		compsVal = rest[0]
	}
	if len(rest) > 1 {
		rulesVal = rest[1]
	}

	// Check whatever is already known: the template syntax, and the static keys
	// (known even when their values are not). Constraints on unknown values
	// wait until they are known.
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
	if template.IsUnknown() || hasUnknown(static) || hasUnknown(compsVal) || hasUnknown(rulesVal) {
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
	ruleset, err := ParseRuleset(rulesVal)
	if err != nil {
		resp.Error = function.NewArgumentFuncError(3, err.Error())
		return
	}
	if err := tpl.CheckRuleset(comps, ruleset); err != nil {
		resp.Error = function.NewArgumentFuncError(3, err.Error())
		return
	}

	out := types.ObjectValueMust(schemeAttrTypes, map[string]attr.Value{
		"template":   template,
		"static":     static,
		"components": ComponentsValue(comps),
		"rules":      RulesetValue(ruleset),
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
			return nil, function.NewArgumentFuncError(argPos, what+"["+quote(k)+"] must not be null")
		}
		out[k] = v.ValueString()
	}
	return out, nil
}

func quote(s string) string { return "\"" + s + "\"" }
