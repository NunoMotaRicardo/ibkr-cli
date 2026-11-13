package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// View says which parts of a provider response a curated command shows.
// Everything is a preference: missing or renamed fields never cause an error.
type View struct {
	ListKeys []string // keys that may hold the row array (first present wins)
	Cols     []string // preferred row columns, in order
	Scalars  []string // preferred keys when the response is a single object
	Hide     []string // keys never shown by default
}

// Result is a normalized, trimmed response ready for output.
type Result struct {
	List bool
	Cols []string
	Rows []map[string]any // flattened rows (list mode) or one row (object mode)
	Meta [][2]any         // top-level scalars next to a list
	Text string           // non-JSON text payload
}

var hiddenSuffixes = []string{"_ex"}

func hidden(key string, v View) bool {
	for _, s := range hiddenSuffixes {
		if strings.HasSuffix(key, s) {
			return true
		}
	}
	for _, h := range v.Hide {
		if h == key {
			return true
		}
	}
	return false
}

// flatten turns nested maps into dotted keys (depth <= 2); other composites
// become compact JSON strings.
func flatten(m map[string]any) map[string]any {
	out := map[string]any{}
	var walk func(prefix string, v any, depth int)
	walk = func(prefix string, v any, depth int) {
		switch x := v.(type) {
		case map[string]any:
			if depth >= 2 || len(x) == 0 {
				out[prefix] = compactJSON(x)
				return
			}
			for k, vv := range x {
				walk(prefix+"."+k, vv, depth+1)
			}
		case []any:
			scalars := true
			for _, e := range x {
				switch e.(type) {
				case map[string]any, []any:
					scalars = false
				}
			}
			if scalars {
				parts := make([]string, len(x))
				for i, e := range x {
					parts[i] = fmtValue(e)
				}
				out[prefix] = strings.Join(parts, "/")
			} else {
				out[prefix] = compactJSON(x)
			}
		default:
			out[prefix] = v
		}
	}
	for k, v := range m {
		switch x := v.(type) {
		case map[string]any:
			if len(x) == 0 {
				out[k] = ""
				continue
			}
			for kk, vv := range x {
				walk(k+"."+kk, vv, 1)
			}
		default:
			walk(k, v, 0)
		}
	}
	return out
}

func compactJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b)
}

func fmtValue(v any) string {
	switch x := v.(type) {
	case nil:
		return "-"
	case string:
		if x == "" {
			return "-"
		}
		return strings.Join(strings.Fields(x), " ")
	case json.Number:
		return x.String()
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(x)
	default:
		return compactJSON(x)
	}
}

func sortedFlatKeys(m map[string]any, v View) []string {
	var ks []string
	for k := range m {
		if !hidden(k, v) {
			ks = append(ks, k)
		}
	}
	sort.Strings(ks)
	return ks
}

// extract normalizes provider data according to a view.
func extract(data any, v View, fields []string, limit int) *Result {
	res := &Result{}
	var rows []any
	var meta map[string]any
	switch d := data.(type) {
	case []any:
		rows = d
		res.List = true
	case map[string]any:
		for _, k := range v.ListKeys {
			if arr, ok := d[k].([]any); ok {
				rows, res.List, meta = arr, true, d
				break
			}
		}
		if !res.List && len(v.ListKeys) == 0 {
			// no hint: use the only array-of-objects value, if there is exactly one
			var found string
			for k, vv := range d {
				if arr, ok := vv.([]any); ok && len(arr) > 0 {
					if _, isObj := arr[0].(map[string]any); isObj {
						if found != "" {
							found = ""
							break
						}
						found = k
					}
				}
			}
			if found != "" {
				rows, res.List, meta = d[found].([]any), true, d
				delete(meta, found)
			}
		}
		if !res.List {
			res.Rows = []map[string]any{flatten(d)}
		}
	case string:
		res.Text = d
		return res
	default:
		res.Text = fmtValue(d)
		return res
	}
	if res.List {
		for _, r := range rows {
			if m, ok := r.(map[string]any); ok {
				res.Rows = append(res.Rows, flatten(m))
			} else {
				res.Rows = append(res.Rows, map[string]any{"value": r})
			}
		}
		if limit > 0 && len(res.Rows) > limit {
			res.Rows = res.Rows[:limit]
		}
		var mk []string
		for k, val := range meta {
			switch val.(type) {
			case map[string]any, []any:
				continue
			}
			if !hidden(k, v) {
				mk = append(mk, k)
			}
		}
		sort.Strings(mk)
		for _, k := range mk {
			res.Meta = append(res.Meta, [2]any{k, meta[k]})
		}
	}
	res.Cols = chooseCols(res.Rows, v, fields, res.List)
	return res
}

func chooseCols(rows []map[string]any, v View, fields []string, list bool) []string {
	if len(fields) > 0 {
		return fields
	}
	present := func(k string) bool {
		for _, r := range rows {
			if _, ok := r[k]; ok {
				return true
			}
		}
		return false
	}
	pref := v.Cols
	if !list {
		pref = v.Scalars
	}
	var cols []string
	for _, c := range pref {
		if present(c) {
			cols = append(cols, c)
		}
	}
	if len(cols) > 0 {
		return cols
	}
	if len(rows) == 0 {
		return nil
	}
	ks := sortedFlatKeys(rows[0], v)
	limit := 8
	if !list {
		limit = 40
	}
	if len(ks) > limit {
		ks = ks[:limit]
	}
	return ks
}

func parseFields(s string) []string {
	var out []string
	for _, f := range strings.Split(s, ",") {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}

func truncate(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n-1]) + "…"
}

// writeText prints the compact human/agent format.
func writeText(w io.Writer, res *Result) {
	if res.Text != "" || (len(res.Rows) == 0 && len(res.Meta) == 0) {
		if res.Text != "" {
			fmt.Fprintln(w, strings.TrimSpace(res.Text))
		} else {
			fmt.Fprintln(w, "(no results)")
		}
		return
	}
	for _, m := range res.Meta {
		fmt.Fprintf(w, "%s: %s\n", m[0], fmtValue(m[1]))
	}
	if !res.List {
		row := res.Rows[0]
		for _, c := range res.Cols {
			if v, ok := row[c]; ok {
				fmt.Fprintf(w, "%s: %s\n", c, truncate(fmtValue(v), 200))
			}
		}
		return
	}
	if len(res.Rows) == 0 {
		fmt.Fprintln(w, "(no results)")
		return
	}
	widths := make([]int, len(res.Cols))
	cells := make([][]string, len(res.Rows))
	for i, c := range res.Cols {
		widths[i] = utf8.RuneCountInString(c)
	}
	for ri, r := range res.Rows {
		cells[ri] = make([]string, len(res.Cols))
		for i, c := range res.Cols {
			s := "-"
			if v, ok := r[c]; ok {
				s = truncate(fmtValue(v), 70)
			}
			cells[ri][i] = s
			if n := utf8.RuneCountInString(s); n > widths[i] {
				widths[i] = n
			}
		}
	}
	line := func(vals []string) {
		var sb strings.Builder
		for i, s := range vals {
			sb.WriteString(s)
			if i < len(vals)-1 {
				sb.WriteString(strings.Repeat(" ", widths[i]-utf8.RuneCountInString(s)+2))
			}
		}
		fmt.Fprintln(w, strings.TrimRight(sb.String(), " "))
	}
	line(res.Cols)
	for _, r := range cells {
		line(r)
	}
}

// writeJSON prints the trimmed object as compact JSON.
func writeJSON(w io.Writer, res *Result) error {
	var out any
	pick := func(r map[string]any) map[string]any {
		o := map[string]any{}
		for _, c := range res.Cols {
			if v, ok := r[c]; ok {
				o[c] = v
			}
		}
		return o
	}
	switch {
	case res.Text != "":
		out = map[string]any{"text": res.Text}
	case res.List:
		rows := make([]map[string]any, 0, len(res.Rows))
		for _, r := range res.Rows {
			rows = append(rows, pick(r))
		}
		if len(res.Meta) > 0 {
			o := map[string]any{"rows": rows}
			for _, m := range res.Meta {
				o[fmt.Sprint(m[0])] = m[1]
			}
			out = o
		} else {
			out = rows
		}
	case len(res.Rows) > 0:
		out = pick(res.Rows[0])
	default:
		out = map[string]any{}
	}
	b, err := json.Marshal(out)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(b))
	return err
}
