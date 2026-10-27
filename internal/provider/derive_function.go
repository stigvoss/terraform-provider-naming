package provider

import (
	"context"
	"fmt"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ function.Function = (*deriveFunction)(nil)

// NewWithFunction returns provider::naming::with, which adds static values to a scheme.
func NewWithFunction() function.Function { return &deriveFunction{name: "with"} }

// NewOverrideFunction returns provider::naming::override, which changes static values of a scheme.
func NewOverrideFunction() function.Function {
	return &deriveFunction{name: "override", override: true}
}

// deriveFunction implements with and override: both return a new scheme whose
// static values differ from the given one. with only adds values, override only
// replaces existing ones, so the intent is visible at the call site.
type deriveFunction struct {
	name     string
	override bool
}

func (f *deriveFunction) Metadata(_ context.Context, _ function.MetadataRequest, resp *function.MetadataResponse) {
	resp.Name = f.name
}

func (f *deriveFunction) Definition(_ context.Context, _ function.DefinitionRequest, resp *function.DefinitionResponse) {
	summary := "Add static values to a scheme"
	description := "Returns a new scheme with the given static values added. Every key must be a placeholder of the template that is not " +
		"static yet; use `override` to change an existing one. The new values are checked against the scheme's component constraints, " +
		"and the constraints and rules are kept, so a family of schemes (for example one per environment) is defined once and " +
		"derived with `with`."
	valuesDescription := "Values for placeholders that are not static yet."
	if f.override {
		summary = "Change static values of a scheme"
		description = "Returns a new scheme with the given static values replaced. Every key must already be a static value of the scheme; " +
			"use `with` to add new ones. The new values are checked against the scheme's component constraints. " +
			"Replacing a value is explicit on purpose, so a typo or a stale value cannot silently win."
		valuesDescription = "New values for existing static keys."
	}
	resp.Definition = function.Definition{
		Summary:     summary,
		Description: description,
		Parameters: []function.Parameter{
			function.ObjectParameter{
				Name:               "scheme",
				Description:        "A scheme, as returned by `scheme`.",
				AttributeTypes:     schemeAttrTypes,
				AllowUnknownValues: true,
			},
			function.MapParameter{
				Name:               "values",
				Description:        valuesDescription,
				ElementType:        types.StringType,
				AllowUnknownValues: true,
			},
		},
		Return: function.ObjectReturn{AttributeTypes: schemeAttrTypes},
	}
}

func (f *deriveFunction) Run(ctx context.Context, req function.RunRequest, resp *function.RunResponse) {
	var obj types.Object
	var values types.Map
	if resp.Error = req.Arguments.Get(ctx, &obj, &values); resp.Error != nil {
		return
	}
	if hasUnknown(obj) || values.IsUnknown() {
		resp.Error = resp.Result.Set(ctx, types.ObjectUnknown(schemeAttrTypes))
		return
	}
	if values.IsNull() {
		resp.Error = function.NewArgumentFuncError(1, "values must not be null")
		return
	}
	ps, ferr := parseScheme(ctx, obj, 0)
	if ferr != nil {
		resp.Error = ferr
		return
	}

	keys := make([]string, 0, len(values.Elements()))
	for k := range values.Elements() {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	fail := func(err error) {
		resp.Error = function.NewArgumentFuncError(1, withScheme(err, ps.template))
	}
	for _, k := range keys {
		_, isStatic := ps.staticVals[k]
		switch {
		case !ps.tpl.Has(k):
			fail(fmt.Errorf("%q is not a placeholder of the template", k))
			return
		case f.override && !isStatic:
			fail(fmt.Errorf("%q is not a static value of this scheme; use with() to add it", k))
			return
		case !f.override && isStatic:
			fail(fmt.Errorf("%q is already a static value of this scheme; use override() to change it", k))
			return
		}
		v, ok := values.Elements()[k].(types.String)
		if !ok || v.IsNull() {
			fail(fmt.Errorf("values[%q] must be a string and not null", k))
			return
		}
		if v.IsUnknown() {
			continue // checked once it is known
		}
		if _, err := CheckValue(v.ValueString(), ps.comps[k]); err != nil {
			fail(fmt.Errorf("component %q (static): %w", k, err))
			return
		}
	}

	merged := make(map[string]attr.Value, len(ps.static.Elements())+len(keys))
	for k, v := range ps.static.Elements() {
		merged[k] = v
	}
	for _, k := range keys {
		merged[k] = values.Elements()[k]
	}
	out := types.ObjectValueMust(schemeAttrTypes, map[string]attr.Value{
		"template":   ps.attrs["template"],
		"static":     types.MapValueMust(types.StringType, merged),
		"components": ps.attrs["components"],
		"rules":      ps.attrs["rules"],
	})
	resp.Error = resp.Result.Set(ctx, out)
}
