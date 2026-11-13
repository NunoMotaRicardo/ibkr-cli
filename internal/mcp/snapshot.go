package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"time"
)

// SnapshotTool is one tool in the committed provider snapshot. InputSchema is a
// signature (types, enums, required, no descriptions) and may be absent when the
// snapshot only knows the tool name.
type SnapshotTool struct {
	Name        string         `json:"name"`
	Summary     string         `json:"summary,omitempty"`
	ReadOnly    *bool          `json:"readOnlyHint,omitempty"`
	InputSchema map[string]any `json:"inputSchema,omitempty"`
}

// Snapshot is specs/provider/tools-snapshot.json.
type Snapshot struct {
	Note        string         `json:"note,omitempty"`
	Server      string         `json:"server"`
	GeneratedAt string         `json:"generated_at"`
	ToolCount   int            `json:"tool_count"`
	Tools       []SnapshotTool `json:"tools"`
}

// Signature strips descriptions/titles/defaults from a schema, keeping what
// affects how a caller must build arguments.
func Signature(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, val := range x {
			switch k {
			case "description", "title", "default", "examples", "$schema", "format", "example":
				continue
			}
			out[k] = Signature(val)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i := range x {
			out[i] = Signature(x[i])
		}
		return out
	default:
		return v
	}
}

// NewSnapshot builds a snapshot from live tools.
func NewSnapshot(server string, tools []Tool, now time.Time) *Snapshot {
	s := &Snapshot{Server: server, GeneratedAt: now.UTC().Format(time.RFC3339), ToolCount: len(tools)}
	for _, t := range tools {
		st := SnapshotTool{Name: t.Name, Summary: FirstSentence(t.Description)}
		if t.Annotations != nil {
			st.ReadOnly = t.Annotations.ReadOnlyHint
		}
		if len(t.InputSchema) > 0 {
			st.InputSchema, _ = Signature(t.InputSchema).(map[string]any)
		}
		s.Tools = append(s.Tools, st)
	}
	sort.Slice(s.Tools, func(i, j int) bool { return s.Tools[i].Name < s.Tools[j].Name })
	return s
}

// WriteSnapshot writes pretty JSON.
func WriteSnapshot(path string, s *Snapshot) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// ReadSnapshot loads a snapshot file.
func ReadSnapshot(path string) (*Snapshot, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Snapshot
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// Diff is the difference between a snapshot and the live tool list.
type Diff struct {
	Added   []string
	Removed []string
	Changed []string // human-readable "name: what changed"
}

// Empty reports no differences.
func (d Diff) Empty() bool { return len(d.Added)+len(d.Removed)+len(d.Changed) == 0 }

// DiffSnapshot compares live tools against a snapshot. Schemas are compared
// only when the snapshot entry carries one.
func DiffSnapshot(s *Snapshot, live []Tool) Diff {
	var d Diff
	old := map[string]SnapshotTool{}
	for _, t := range s.Tools {
		old[t.Name] = t
	}
	seen := map[string]bool{}
	for _, t := range live {
		seen[t.Name] = true
		o, ok := old[t.Name]
		if !ok {
			d.Added = append(d.Added, t.Name)
			continue
		}
		if len(o.InputSchema) > 0 {
			cur, _ := Signature(t.InputSchema).(map[string]any)
			if !reflect.DeepEqual(normalize(o.InputSchema), normalize(cur)) {
				d.Changed = append(d.Changed, t.Name+": "+schemaDelta(o.InputSchema, cur))
			}
		}
		if o.ReadOnly != nil && t.Annotations != nil && t.Annotations.ReadOnlyHint != nil && *o.ReadOnly != *t.Annotations.ReadOnlyHint {
			d.Changed = append(d.Changed, fmt.Sprintf("%s: readOnlyHint %v -> %v", t.Name, *o.ReadOnly, *t.Annotations.ReadOnlyHint))
		}
	}
	for n := range old {
		if !seen[n] {
			d.Removed = append(d.Removed, n)
		}
	}
	sort.Strings(d.Added)
	sort.Strings(d.Removed)
	sort.Strings(d.Changed)
	return d
}

// normalize round-trips through JSON so []string vs []any etc. compare equal.
func normalize(v any) any {
	b, _ := json.Marshal(v)
	var out any
	_ = json.Unmarshal(b, &out)
	if m, ok := out.(map[string]any); ok {
		if r, ok := m["required"].([]any); ok {
			sort.Slice(r, func(i, j int) bool { return fmt.Sprint(r[i]) < fmt.Sprint(r[j]) })
		}
	}
	return out
}

func schemaDelta(a, b map[string]any) string {
	pa, pb := schemaProps(a), schemaProps(b)
	var parts []string
	for _, k := range sortedKeys(pb) {
		if _, ok := pa[k]; !ok {
			parts = append(parts, "+param "+k)
		}
	}
	for _, k := range sortedKeys(pa) {
		if _, ok := pb[k]; !ok {
			parts = append(parts, "-param "+k)
		}
	}
	for _, k := range sortedKeys(pa) {
		if v, ok := pb[k]; ok && !reflect.DeepEqual(normalize(pa[k]), normalize(v)) {
			parts = append(parts, "~param "+k)
		}
	}
	ra, rb := schemaRequired(a), schemaRequired(b)
	if !reflect.DeepEqual(ra, rb) {
		parts = append(parts, "required changed")
	}
	if len(parts) == 0 {
		parts = append(parts, "schema changed")
	}
	return strings.Join(parts, ", ")
}
