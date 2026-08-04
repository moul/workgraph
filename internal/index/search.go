package index

import "strings"

// Search returns objects whose id, title, or summary contains the query
// (case-insensitive). An empty query returns everything. This is the
// deterministic-filter fallback; a SQLite FTS layer can supersede it later
// without changing the result contract.
func Search(objs []ObjectLine, query string) []ObjectLine {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return objs
	}
	var out []ObjectLine
	for _, o := range objs {
		hay := strings.ToLower(o.ID + " " + o.Title + " " + o.Summary + " " + o.Status)
		if strings.Contains(hay, q) {
			out = append(out, o)
		}
	}
	return out
}
