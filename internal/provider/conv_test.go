package provider

import (
	"context"
	"math/big"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func obj(fields map[string]attr.Value) attr.Value {
	attrTypes := map[string]attr.Type{}
	for k, v := range fields {
		attrTypes[k] = v.Type(context.Background())
	}
	return types.ObjectValueMust(attrTypes, fields)
}

func num(n int) attr.Value { return types.NumberValue(big.NewFloat(float64(n))) }

func TestParseComponents(t *testing.T) {
	good := obj(map[string]attr.Value{
		"name": obj(map[string]attr.Value{
			"max_length": num(3),
			"overflow":   types.StringValue("truncate"),
			"hash":       num(2),
			"allowed":    types.TupleValueMust([]attr.Type{types.StringType}, []attr.Value{types.StringValue("a")}),
		}),
		"env": obj(map[string]attr.Value{}),
	})
	got, err := ParseComponents(good)
	if err != nil {
		t.Fatal(err)
	}
	n := got["name"]
	if *n.MaxLength != 3 || n.Overflow != "truncate" || n.Hash != 2 || len(n.Allowed) != 1 {
		t.Fatalf("unexpected spec: %+v", n)
	}
	if got["env"].MaxLength != nil {
		t.Fatal("empty spec must be unlimited")
	}

	tests := []struct {
		name    string
		in      attr.Value
		wantErr string
	}{
		{"unknown field", obj(map[string]attr.Value{"name": obj(map[string]attr.Value{"max_lenght": num(3)})}), "unknown field"},
		{"fractional number", obj(map[string]attr.Value{"name": obj(map[string]attr.Value{"max_length": types.NumberValue(big.NewFloat(2.5))})}), "whole number"},
		{"wrong type", obj(map[string]attr.Value{"name": obj(map[string]attr.Value{"max_length": types.StringValue("3")})}), "must be a number"},
		{"hash without truncate", obj(map[string]attr.Value{"name": obj(map[string]attr.Value{"hash": num(2)})}), "only applies"},
		{"not an object", types.StringValue("x"), "must be an object"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseComponents(tc.in)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("want error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestParseRules(t *testing.T) {
	r, err := ParseRules(obj(map[string]attr.Value{
		"lowercase":  types.BoolValue(true),
		"remove":     types.StringValue("[^a-z]"),
		"max_length": num(24),
		"fit":        types.StringValue("name"),
		"components": obj(map[string]attr.Value{"type": obj(map[string]attr.Value{"max_length": num(2)})}),
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !r.Lowercase || r.Remove != "[^a-z]" || *r.MaxLength != 24 || r.Fit != "name" || *r.Components["type"].MaxLength != 2 {
		t.Fatalf("unexpected rules: %+v", r)
	}

	tests := []struct {
		name    string
		in      attr.Value
		wantErr string
	}{
		{"typo", obj(map[string]attr.Value{"lowercse": types.BoolValue(true)}), "unknown field"},
		{"fit without max_length", obj(map[string]attr.Value{"fit": types.StringValue("name")}), "fit requires max_length"},
		{"bad remove", obj(map[string]attr.Value{"remove": types.StringValue("(")}), "invalid remove"},
		{"override field not allowed", obj(map[string]attr.Value{"components": obj(map[string]attr.Value{"type": obj(map[string]attr.Value{"overflow": types.StringValue("truncate")})})}), "unknown field"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseRules(tc.in)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("want error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestHasUnknown(t *testing.T) {
	known := types.MapValueMust(types.StringType, map[string]attr.Value{"a": types.StringValue("x")})
	unknownElem := types.MapValueMust(types.StringType, map[string]attr.Value{"a": types.StringUnknown()})
	if hasUnknown(known) {
		t.Fatal("known map reported unknown")
	}
	if !hasUnknown(unknownElem) {
		t.Fatal("unknown element not detected")
	}
	if !hasUnknown(obj(map[string]attr.Value{"m": unknownElem})) {
		t.Fatal("nested unknown not detected")
	}
}
