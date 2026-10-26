package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

var _ function.Function = (*formatFunction)(nil)

// NewFormatFunction returns the provider::naming::format function.
func NewFormatFunction() function.Function { return &formatFunction{} }

type formatFunction struct{}

type namingConfig struct {
	Template types.String `tfsdk:"template"`
	Static   types.Map    `tfsdk:"static"`
}

func (f *formatFunction) Metadata(_ context.Context, _ function.MetadataRequest, resp *function.MetadataResponse) {
	resp.Name = "format"
}

func (f *formatFunction) Definition(_ context.Context, _ function.DefinitionRequest, resp *function.DefinitionResponse) {
	resp.Definition = function.Definition{
		Summary:     "Render a resource name from a naming template",
		Description: "Fills `config.template` with `config.static` plus the variable parts in `variables`. Define `config` once (a local or module output) and pass it to every call.",
		Parameters: []function.Parameter{
			function.ObjectParameter{
				Name:               "config",
				Description:        "Object with `template` (string with `{key}` placeholders) and `static` (map of module-wide values; use {} for none).",
				AttributeTypes:     schemeAttrTypes,
				AllowUnknownValues: true,
			},
			function.MapParameter{
				Name:               "variables",
				Description:        "Values for the placeholders not set in `config.static`.",
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

	unknown := func() {
		resp.Error = resp.Result.Set(ctx, types.StringUnknown())
	}
	if obj.IsUnknown() || vars.IsUnknown() {
		unknown()
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

	var cfg namingConfig
	if diags := obj.As(ctx, &cfg, basetypes.ObjectAsOptions{}); diags.HasError() {
		resp.Error = function.FuncErrorFromDiags(ctx, diags)
		return
	}
	if cfg.Template.IsUnknown() || cfg.Static.IsUnknown() {
		unknown()
		return
	}
	if cfg.Template.IsNull() || cfg.Static.IsNull() {
		resp.Error = function.NewArgumentFuncError(0, "config.template and config.static must not be null")
		return
	}

	static, ok, err := stringMap(ctx, cfg.Static, 0, "config.static")
	if err != nil {
		resp.Error = err
		return
	}
	if !ok {
		unknown()
		return
	}
	call, ok, err := stringMap(ctx, vars, 1, "variables")
	if err != nil {
		resp.Error = err
		return
	}
	if !ok {
		unknown()
		return
	}

	tpl, perr := ParseTemplate(cfg.Template.ValueString())
	if perr != nil {
		resp.Error = function.NewArgumentFuncError(0, perr.Error())
		return
	}
	staticKeys := make([]string, 0, len(static))
	for k := range static {
		staticKeys = append(staticKeys, k)
	}
	if unused := tpl.UnusedKeys(staticKeys); len(unused) > 0 {
		resp.Error = function.NewArgumentFuncError(0, fmt.Sprintf("config.static key(s) %s are not used by the template %q", quoteAll(unused), cfg.Template.ValueString()))
		return
	}

	name, rerr := tpl.Render(static, call)
	if rerr != nil {
		resp.Error = function.NewArgumentFuncError(1, rerr.Error())
		return
	}
	resp.Error = resp.Result.Set(ctx, name)
}

// stringMap converts m to a Go map. ok is false when any element is unknown.
func stringMap(ctx context.Context, m types.Map, argPos int64, what string) (out map[string]string, ok bool, ferr *function.FuncError) {
	var raw map[string]types.String
	if diags := m.ElementsAs(ctx, &raw, false); diags.HasError() {
		return nil, false, function.FuncErrorFromDiags(ctx, diags)
	}
	out = make(map[string]string, len(raw))
	for k, v := range raw {
		switch {
		case v.IsUnknown():
			return nil, false, nil
		case v.IsNull():
			return nil, false, function.NewArgumentFuncError(argPos, fmt.Sprintf("%s[%q] must not be null", what, k))
		}
		out[k] = v.ValueString()
	}
	return out, true, nil
}
