// Package redact holds the secret-shaped patterns Workgraph uses both to warn
// (validation) and to scrub (capsule generation). Keeping them in one place
// means the scanner and the redactor never drift.
package redact

import "regexp"

// Pattern is a named secret shape.
type Pattern struct {
	Name string
	RE   *regexp.Regexp
}

// Patterns are obvious credential shapes. This is a lightweight heuristic, not a
// guarantee — but it catches the common mistakes before they reach a target
// repo or a public push.
var Patterns = []Pattern{
	{"AWS access key", regexp.MustCompile(`AKIA[0-9A-Z]{16}`)},
	{"GitHub token", regexp.MustCompile(`gh[pousr]_[0-9A-Za-z]{36,}`)},
	{"Slack token", regexp.MustCompile(`xox[baprs]-[0-9A-Za-z-]{10,}`)},
	{"private key", regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`)},
	{"generic secret assignment", regexp.MustCompile(`(?i)(api[_-]?key|secret|password|token)\s*[:=]\s*['"][0-9A-Za-z/+_\-]{20,}['"]`)},
}

// Redact replaces every secret-shaped match in s with a labeled marker and
// reports which pattern names fired. It is safe to run on capsule text before
// writing it into a target repo.
func Redact(s string) (string, []string) {
	var hit []string
	for _, p := range Patterns {
		if p.RE.MatchString(s) {
			hit = append(hit, p.Name)
			s = p.RE.ReplaceAllString(s, "[REDACTED:"+p.Name+"]")
		}
	}
	return s, hit
}

// Matches reports the pattern names that fire anywhere in s (no substitution).
func Matches(s string) []string {
	var hit []string
	for _, p := range Patterns {
		if p.RE.MatchString(s) {
			hit = append(hit, p.Name)
		}
	}
	return hit
}
