package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// parsedScheme is a scheme object (as returned by scheme) read back into Go
// values. attrs keeps the original attribute values for building a derived
// scheme without converting them twice.
type parsedScheme struct {
	template   string
	tpl        *Template
	static     types.Map
	staticVals map[string]string
	comps      map[string]ComponentSpec
	ruleset    Ruleset
	attrs      map[string]attr.Value
}

// parseScheme reads a scheme argument. arg is the argument position used in
// errors. The object must not contain unknown values.
func parseScheme(ctx context.Context, obj types.Object, arg int64) (*parsedScheme, *function.FuncError) {
	if obj.IsNull() {
		return nil, function.NewArgumentFuncError(arg, "the scheme must not be null")
	}
	attrs := obj.Attributes()
	template, _ := attrs["template"].(types.String)
	static, _ := attrs["static"].(types.Map)
	if template.IsNull() || static.IsNull() {
		return nil, function.NewArgumentFuncError(arg, "config.template and config.static must not be null")
	}
	staticVals, ferr := stringMap(ctx, static, arg, "config.static")
	if ferr != nil {
		return nil, ferr
	}
	comps, err := ParseComponents(attrs["components"])
	if err != nil {
		return nil, function.NewArgumentFuncError(arg, "config."+err.Error())
	}
	ruleset, err := ParseRuleset(attrs["rules"])
	if err != nil {
		return nil, function.NewArgumentFuncError(arg, "config."+err.Error())
	}
	tpl, err := ParseTemplate(template.ValueString())
	if err != nil {
		return nil, function.NewArgumentFuncError(arg, err.Error())
	}
	return &parsedScheme{
		template:   template.ValueString(),
		tpl:        tpl,
		static:     static,
		staticVals: staticVals,
		comps:      comps,
		ruleset:    ruleset,
		attrs:      attrs,
	}, nil
}
