package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Param is one input parameter, flattened for display.
type Param struct {
	Name        string
	Type        string
	Required    bool
	Enum        []string
	Description string
}

func schemaProps(schema map[string]any) map[string]any {
	p, _ := schema["properties"].(map[string]any)
	return p
}

func schemaRequired(schema map[string]any) map[string]bool {
	out := map[string]bool{}
	if r, ok := schema["required"].([]any); ok {
		for _, v := range r {
			if s, ok := v.(string); ok {
				out[s] = true
			}
		}
	}
	return out
}

// propType returns the primary JSON type of a property schema, looking through
// type arrays and anyOf/oneOf.
func propType(p map[string]any) string {
	switch t := p["type"].(type) {
	case string:
		return t
	case []any:
		for _, v := range t {
			if s, ok := v.(string); ok && s != "null" {
				return s
			}
		}
	}
	for _, k := range []string{"anyOf", "oneOf"} {
		if alts, ok := p[k].([]any); ok {
			for _, a := range alts {
				if m, ok := a.(map[string]any); ok {
					if t := propType(m); t != "" && t != "null" {
						return t
					}
				}
			}
		}
	}
	if _, ok := p["enum"]; ok {
		return "string"
	}
	return ""
}

func enumOf(p map[string]any) []string {
	var out []string
	if e, ok := p["enum"].([]any); ok {
		for _, v := range e {
			out = append(out, fmt.Sprint(v))
		}
	}
	if len(out) == 0 {
		if items, ok := p["items"].(map[string]any); ok {
			return enumOf(items)
		}
	}
	return out
}

// Params lists a tool's parameters, required first then alphabetical.
func Params(schema map[string]any) []Param {
	props := schemaProps(schema)
	req := schemaRequired(schema)
	var out []Param
	for name, v := range props {
		m, _ := v.(map[string]any)
		t := propType(m)
		if t == "array" {
			if items, ok := m["items"].(map[string]any); ok {
				t = "array<" + orStr(propType(items), "any") + ">"
			}
		}
		d, _ := m["description"].(string)
		out = append(out, Param{Name: name, Type: orStr(t, "any"), Required: req[name], Enum: enumOf(m), Description: FirstSentence(d)})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Required != out[j].Required {
			return out[i].Required
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func orStr(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

// FirstSentence returns the first sentence/line of s, capped at 140 chars.
func FirstSentence(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "\n"); i >= 0 {
		s = s[:i]
	}
	if i := strings.Index(s, ". "); i >= 0 {
		s = s[:i+1]
	}
	if len(s) > 140 {
		s = strings.TrimSpace(s[:137]) + "..."
	}
	return s
}

// BuildArgs turns key=value pairs (and an optional JSON object) into typed tool
// arguments using the tool's input schema. kv entries override jsonArgs.
// With a nil/empty schema, values are inferred (ints, bools, JSON), except keys
// ending in "_ex" which always stay strings.
func BuildArgs(schema map[string]any, kv []string, jsonArgs string) (map[string]any, error) {
	args := map[string]any{}
	if strings.TrimSpace(jsonArgs) != "" {
		d := json.NewDecoder(strings.NewReader(jsonArgs))
		d.UseNumber()
		if err := d.Decode(&args); err != nil {
			return nil, fmt.Errorf("--args-json is not a valid JSON object: %w", err)
		}
	}
	props := schemaProps(schema)
	for _, pair := range kv {
		i := strings.Index(pair, "=")
		if i <= 0 {
			return nil, fmt.Errorf("argument %q must look like key=value", pair)
		}
		key, raw := pair[:i], pair[i+1:]
		var pm map[string]any
		if props != nil {
			v, ok := props[key]
			if !ok {
				if ap, ok := schema["additionalProperties"].(bool); ok && ap {
					args[key] = inferValue(key, raw)
					continue
				}
				return nil, fmt.Errorf("unknown parameter %q; valid: %s", key, strings.Join(sortedKeys(props), ", "))
			}
			pm, _ = v.(map[string]any)
		}
		val, err := coerce(key, raw, pm)
		if err != nil {
			return nil, err
		}
		if pm != nil && propType(pm) == "array" {
			if prev, ok := args[key].([]any); ok {
				if add, ok := val.([]any); ok {
					val = append(prev, add...)
				}
			}
		}
		args[key] = val
	}
	if props != nil {
		var missing []string
		for r := range schemaRequired(schema) {
			if _, ok := args[r]; !ok {
				missing = append(missing, r)
			}
		}
		if len(missing) > 0 {
			sort.Strings(missing)
			return nil, fmt.Errorf("missing required parameter(s): %s", strings.Join(missing, ", "))
		}
	}
	return args, nil
}

func sortedKeys(m map[string]any) []string {
	var k []string
	for x := range m {
		k = append(k, x)
	}
	sort.Strings(k)
	return k
}

func inferValue(key, raw string) any {
	if strings.HasSuffix(key, "_ex") {
		return raw
	}
	if raw == "true" || raw == "false" {
		return raw == "true"
	}
	if n, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return n
	}
	if f, err := strconv.ParseFloat(raw, 64); err == nil && !strings.ContainsAny(raw, "eE") {
		return f
	}
	if raw != "" && (raw[0] == '[' || raw[0] == '{') {
		var v any
		if json.Unmarshal([]byte(raw), &v) == nil {
			return v
		}
	}
	return raw
}

func coerce(key, raw string, pm map[string]any) (any, error) {
	if pm == nil {
		return inferValue(key, raw), nil
	}
	t := propType(pm)
	var v any
	switch t {
	case "integer":
		n, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parameter %s must be an integer, got %q", key, raw)
		}
		v = n
	case "number":
		f, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
		if err != nil {
			return nil, fmt.Errorf("parameter %s must be a number, got %q", key, raw)
		}
		v = f
	case "boolean":
		b, err := strconv.ParseBool(strings.TrimSpace(raw))
		if err != nil {
			return nil, fmt.Errorf("parameter %s must be true or false, got %q", key, raw)
		}
		v = b
	case "array":
		items, _ := pm["items"].(map[string]any)
		var elems []string
		if s := strings.TrimSpace(raw); strings.HasPrefix(s, "[") {
			var arr []any
			d := json.NewDecoder(strings.NewReader(s))
			d.UseNumber()
			if err := d.Decode(&arr); err != nil {
				return nil, fmt.Errorf("parameter %s: invalid JSON array: %w", key, err)
			}
			return arr, nil
		}
		if raw != "" {
			for _, e := range strings.Split(raw, ",") {
				elems = append(elems, strings.TrimSpace(e))
			}
		}
		out := make([]any, 0, len(elems))
		for _, e := range elems {
			cv, err := coerce(key, e, items)
			if err != nil {
				return nil, err
			}
			out = append(out, cv)
		}
		return out, nil
	case "object":
		var o any
		d := json.NewDecoder(bytes.NewReader([]byte(raw)))
		d.UseNumber()
		if err := d.Decode(&o); err != nil {
			return nil, fmt.Errorf("parameter %s must be a JSON object: %w", key, err)
		}
		return o, nil
	case "string":
		v = raw
	default:
		v = inferValue(key, raw)
	}
	if en := enumOf(pm); len(en) > 0 && t != "array" {
		s := fmt.Sprint(v)
		for _, e := range en {
			if e == s {
				return v, nil
			}
		}
		for _, e := range en {
			if strings.EqualFold(e, s) {
				return e, nil
			}
		}
		return nil, fmt.Errorf("parameter %s must be one of: %s (got %q)", key, strings.Join(en, ", "), raw)
	}
	return v, nil
}

// PickParam returns the first candidate name that is a property of the schema,
// or the first candidate when the schema is unknown or has none of them.
func PickParam(schema map[string]any, candidates ...string) string {
	props := schemaProps(schema)
	for _, c := range candidates {
		if _, ok := props[c]; ok {
			return c
		}
	}
	return candidates[0]
}

// HasParam reports whether the schema declares the property.
func HasParam(schema map[string]any, name string) bool {
	_, ok := schemaProps(schema)[name]
	return ok
}
