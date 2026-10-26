package provider

import (
	"context"
	"fmt"
	"math/big"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// Conversions between Terraform values and the specs in spec.go. Specs arrive
// as dynamically typed objects so that every field can be optional.

var componentFields = []string{"min_length", "max_length", "allowed", "pattern", "overflow", "hash"}
var overrideFields = []string{"min_length", "max_length", "allowed"}
var ruleFields = []string{"lowercase", "remove", "min_length", "max_length", "fit", "components"}

var componentAttrTypes = map[string]attr.Type{
	"min_length": types.NumberType,
	"max_length": types.NumberType,
	"allowed":    types.ListType{ElemType: types.StringType},
	"pattern":    types.StringType,
	"overflow":   types.StringType,
	"hash":       types.NumberType,
}

var overrideAttrTypes = map[string]attr.Type{
	"min_length": types.NumberType,
	"max_length": types.NumberType,
	"allowed":    types.ListType{ElemType: types.StringType},
}

var componentObjType = types.ObjectType{AttrTypes: componentAttrTypes}
var overrideObjType = types.ObjectType{AttrTypes: overrideAttrTypes}

// schemeAttrTypes is the shape of scheme's result and format's config parameter.
var schemeAttrTypes = map[string]attr.Type{
	"template":   types.StringType,
	"static":     types.MapType{ElemType: types.StringType},
	"components": types.MapType{ElemType: componentObjType},
}

var rulesAttrTypes = map[string]attr.Type{
	"lowercase":  types.BoolType,
	"remove":     types.StringType,
	"min_length": types.NumberType,
	"max_length": types.NumberType,
	"fit":        types.StringType,
	"components": types.MapType{ElemType: overrideObjType},
}

func unwrap(v attr.Value) attr.Value {
	for {
		d, ok := v.(basetypes.DynamicValue)
		if !ok {
			return v
		}
		if d.IsUnknown() || d.IsNull() {
			return d
		}
		v = d.UnderlyingValue()
	}
}

// hasUnknown reports whether v, or anything nested in it, is unknown.
func hasUnknown(v attr.Value) bool {
	v = unwrap(v)
	if v.IsUnknown() {
		return true
	}
	if v.IsNull() {
		return false
	}
	switch t := v.(type) {
	case basetypes.ObjectValue:
		for _, e := range t.Attributes() {
			if hasUnknown(e) {
				return true
			}
		}
	case basetypes.MapValue:
		for _, e := range t.Elements() {
			if hasUnknown(e) {
				return true
			}
		}
	case basetypes.ListValue:
		for _, e := range t.Elements() {
			if hasUnknown(e) {
				return true
			}
		}
	case basetypes.SetValue:
		for _, e := range t.Elements() {
			if hasUnknown(e) {
				return true
			}
		}
	case basetypes.TupleValue:
		for _, e := range t.Elements() {
			if hasUnknown(e) {
				return true
			}
		}
	}
	return false
}

// fields returns the attributes of an object or the elements of a map.
func fields(v attr.Value, what string) (map[string]attr.Value, error) {
	switch t := unwrap(v).(type) {
	case basetypes.ObjectValue:
		return t.Attributes(), nil
	case basetypes.MapValue:
		return t.Elements(), nil
	default:
		return nil, fmt.Errorf("%s must be an object, got %s", what, t.Type(context.Background()))
	}
}

func checkFields(m map[string]attr.Value, allowed []string, what string) error {
	ok := map[string]bool{}
	for _, f := range allowed {
		ok[f] = true
	}
	var bad []string
	for k := range m {
		if !ok[k] {
			bad = append(bad, k)
		}
	}
	if len(bad) > 0 {
		sort.Strings(bad)
		return fmt.Errorf("%s has unknown field(s) %s; valid fields are %s", what, quoteAll(bad), quoteAll(allowed))
	}
	return nil
}

func attrInt(m map[string]attr.Value, k, what string) (*int, error) {
	v, ok := m[k]
	if !ok {
		return nil, nil
	}
	switch t := unwrap(v).(type) {
	case basetypes.NumberValue:
		if t.IsNull() {
			return nil, nil
		}
		f := t.ValueBigFloat()
		if !f.IsInt() {
			return nil, fmt.Errorf("%s.%s must be a whole number", what, k)
		}
		i, _ := f.Int64()
		n := int(i)
		return &n, nil
	case basetypes.Int64Value:
		if t.IsNull() {
			return nil, nil
		}
		n := int(t.ValueInt64())
		return &n, nil
	default:
		return nil, fmt.Errorf("%s.%s must be a number", what, k)
	}
}

func attrString(m map[string]attr.Value, k, what string) (string, error) {
	v, ok := m[k]
	if !ok {
		return "", nil
	}
	s, ok := unwrap(v).(basetypes.StringValue)
	if !ok {
		return "", fmt.Errorf("%s.%s must be a string", what, k)
	}
	return s.ValueString(), nil
}

func attrBool(m map[string]attr.Value, k, what string) (bool, error) {
	v, ok := m[k]
	if !ok {
		return false, nil
	}
	b, ok := unwrap(v).(basetypes.BoolValue)
	if !ok {
		return false, fmt.Errorf("%s.%s must be a bool", what, k)
	}
	return b.ValueBool(), nil
}

func attrStrings(m map[string]attr.Value, k, what string) ([]string, error) {
	v, ok := m[k]
	if !ok {
		return nil, nil
	}
	var elems []attr.Value
	switch t := unwrap(v).(type) {
	case basetypes.ListValue:
		elems = t.Elements()
	case basetypes.TupleValue:
		elems = t.Elements()
	case basetypes.SetValue:
		elems = t.Elements()
	default:
		return nil, fmt.Errorf("%s.%s must be a list of strings", what, k)
	}
	out := make([]string, 0, len(elems))
	for _, e := range elems {
		s, ok := unwrap(e).(basetypes.StringValue)
		if !ok || s.IsNull() {
			return nil, fmt.Errorf("%s.%s must be a list of strings", what, k)
		}
		out = append(out, s.ValueString())
	}
	return out, nil
}

func parseSpec(v attr.Value, allowed []string, what string) (ComponentSpec, error) {
	var s ComponentSpec
	m, err := fields(v, what)
	if err != nil {
		return s, err
	}
	if err := checkFields(m, allowed, what); err != nil {
		return s, err
	}
	if s.MinLength, err = attrInt(m, "min_length", what); err != nil {
		return s, err
	}
	if s.MaxLength, err = attrInt(m, "max_length", what); err != nil {
		return s, err
	}
	if s.Allowed, err = attrStrings(m, "allowed", what); err != nil {
		return s, err
	}
	if s.Pattern, err = attrString(m, "pattern", what); err != nil {
		return s, err
	}
	if s.Overflow, err = attrString(m, "overflow", what); err != nil {
		return s, err
	}
	h, err := attrInt(m, "hash", what)
	if err != nil {
		return s, err
	}
	if h != nil {
		s.Hash = *h
	}
	if err := s.Validate(); err != nil {
		return s, fmt.Errorf("%s: %w", what, err)
	}
	return s, nil
}

// ParseComponents reads a map of component name -> spec. A null value means none.
func ParseComponents(v attr.Value) (map[string]ComponentSpec, error) {
	if v == nil || unwrap(v).IsNull() {
		return nil, nil
	}
	m, err := fields(v, "components")
	if err != nil {
		return nil, err
	}
	out := make(map[string]ComponentSpec, len(m))
	for k, e := range m {
		if unwrap(e).IsNull() {
			out[k] = ComponentSpec{}
			continue
		}
		if out[k], err = parseSpec(e, componentFields, "components."+k); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// ParseRules reads a rules object. A null value means no rules.
func ParseRules(v attr.Value) (Rules, error) {
	var r Rules
	if v == nil || unwrap(v).IsNull() {
		return r, nil
	}
	m, err := fields(v, "rules")
	if err != nil {
		return r, err
	}
	if err = checkFields(m, ruleFields, "rules"); err != nil {
		return r, err
	}
	if r.Lowercase, err = attrBool(m, "lowercase", "rules"); err != nil {
		return r, err
	}
	if r.Remove, err = attrString(m, "remove", "rules"); err != nil {
		return r, err
	}
	if r.MinLength, err = attrInt(m, "min_length", "rules"); err != nil {
		return r, err
	}
	if r.MaxLength, err = attrInt(m, "max_length", "rules"); err != nil {
		return r, err
	}
	if r.Fit, err = attrString(m, "fit", "rules"); err != nil {
		return r, err
	}
	if c, ok := m["components"]; ok && !unwrap(c).IsNull() {
		cm, err := fields(c, "rules.components")
		if err != nil {
			return r, err
		}
		r.Components = make(map[string]ComponentSpec, len(cm))
		for k, e := range cm {
			if r.Components[k], err = parseSpec(e, overrideFields, "rules.components."+k); err != nil {
				return r, err
			}
		}
	}
	return r, r.Validate()
}

func numberOrNull(n *int) attr.Value {
	if n == nil {
		return types.NumberNull()
	}
	return types.NumberValue(big.NewFloat(float64(*n)))
}

func stringOrNull(s string) attr.Value {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

func stringsOrNull(ss []string) attr.Value {
	if len(ss) == 0 {
		return types.ListNull(types.StringType)
	}
	elems := make([]attr.Value, len(ss))
	for i, s := range ss {
		elems[i] = types.StringValue(s)
	}
	return types.ListValueMust(types.StringType, elems)
}

// ComponentsValue converts specs to the normalized, fully typed value
// scheme returns, with null for unset fields.
func ComponentsValue(specs map[string]ComponentSpec) types.Map {
	elems := make(map[string]attr.Value, len(specs))
	for k, s := range specs {
		var hash *int
		if s.Hash > 0 {
			hash = &s.Hash
		}
		elems[k] = types.ObjectValueMust(componentAttrTypes, map[string]attr.Value{
			"min_length": numberOrNull(s.MinLength),
			"max_length": numberOrNull(s.MaxLength),
			"allowed":    stringsOrNull(s.Allowed),
			"pattern":    stringOrNull(s.Pattern),
			"overflow":   stringOrNull(s.Overflow),
			"hash":       numberOrNull(hash),
		})
	}
	return types.MapValueMust(componentObjType, elems)
}

// RulesValue converts rules to the normalized, fully typed value rules returns.
func RulesValue(r Rules) types.Object {
	comps := make(map[string]attr.Value, len(r.Components))
	for k, s := range r.Components {
		comps[k] = types.ObjectValueMust(overrideAttrTypes, map[string]attr.Value{
			"min_length": numberOrNull(s.MinLength),
			"max_length": numberOrNull(s.MaxLength),
			"allowed":    stringsOrNull(s.Allowed),
		})
	}
	return types.ObjectValueMust(rulesAttrTypes, map[string]attr.Value{
		"lowercase":  types.BoolValue(r.Lowercase),
		"remove":     stringOrNull(r.Remove),
		"min_length": numberOrNull(r.MinLength),
		"max_length": numberOrNull(r.MaxLength),
		"fit":        stringOrNull(r.Fit),
		"components": types.MapValueMust(overrideObjType, comps),
	})
}
