package mcp

import "strings"

// Kind classifies a tool by what it can change.
type Kind int

const (
	KindRead  Kind = iota // no side effects
	KindWrite             // changes account-adjacent state (alerts, watchlists, instructions, feedback)
	KindOrder             // can place, change or cancel a live order
)

func (k Kind) String() string {
	switch k {
	case KindRead:
		return "read"
	case KindWrite:
		return "write"
	default:
		return "order"
	}
}

var readPrefixes = []string{"get_", "search_", "list_", "preview_", "whats_new", "describe"}

var orderVerbs = []string{"submit", "modify", "cancel", "place", "replace", "amend"}

// IsOrderName reports whether a tool name looks like a live-order mutation.
// Order-instruction tools (reviewable, not live) are plain writes.
func IsOrderName(name string) bool {
	n := strings.ToLower(name)
	if strings.Contains(n, "instruction") {
		return false
	}
	for _, v := range orderVerbs {
		if strings.HasPrefix(n, v+"_") && strings.Contains(n, "order") {
			return true
		}
		if v == "cancel" && strings.HasPrefix(n, "cancel") {
			return true
		}
	}
	return n == "submit_order" || n == "modify_order"
}

func nameLooksRead(name string) bool {
	n := strings.ToLower(name)
	for _, p := range readPrefixes {
		if strings.HasPrefix(n, p) {
			return true
		}
	}
	return false
}

// Classify decides read / write / order. Order-looking names always win. Then
// MCP annotations are used when present (an explicit non-read-only hint or a
// destructive hint means write), else the name heuristic applies.
func Classify(t Tool) Kind {
	if IsOrderName(t.Name) {
		return KindOrder
	}
	if a := t.Annotations; a != nil {
		if a.DestructiveHint != nil && *a.DestructiveHint {
			return KindWrite
		}
		if a.ReadOnlyHint != nil {
			if *a.ReadOnlyHint {
				return KindRead
			}
			return KindWrite
		}
	}
	if nameLooksRead(t.Name) {
		return KindRead
	}
	return KindWrite
}

// ClassifyName classifies a tool known only by name.
func ClassifyName(name string) Kind { return Classify(Tool{Name: name}) }
