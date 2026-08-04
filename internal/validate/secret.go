package validate

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/moul/workgraph/internal/store"
)

// secretPatterns are obvious credential shapes. This is a lightweight scan, not
// a guarantee — but it should catch the common mistakes before they are pushed.
var secretPatterns = []struct {
	name string
	re   *regexp.Regexp
}{
	{"AWS access key", regexp.MustCompile(`AKIA[0-9A-Z]{16}`)},
	{"GitHub token", regexp.MustCompile(`gh[pousr]_[0-9A-Za-z]{36,}`)},
	{"Slack token", regexp.MustCompile(`xox[baprs]-[0-9A-Za-z-]{10,}`)},
	{"private key", regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`)},
	{"generic secret assignment", regexp.MustCompile(`(?i)(api[_-]?key|secret|password|token)\s*[:=]\s*['"][0-9A-Za-z/+_\-]{20,}['"]`)},
}

// secretScan walks committed source files looking for obvious secrets. Secret
// *names*, env var names, and vault paths are fine; literal high-entropy values
// are not.
func secretScan(w *store.Workspace, r *Report) {
	roots := []string{"projects", "inbox", "workers", "events", "ontologies"}
	for _, sub := range roots {
		filepath.Walk(filepath.Join(w.Root, sub), func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if ext != ".md" && ext != ".jsonl" && ext != ".yaml" && ext != ".json" {
				return nil
			}
			f, err := os.Open(path)
			if err != nil {
				return nil
			}
			defer f.Close()
			rel, _ := filepath.Rel(w.Root, path)
			sc := bufio.NewScanner(f)
			sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
			ln := 0
			for sc.Scan() {
				ln++
				line := sc.Text()
				for _, p := range secretPatterns {
					if p.re.MatchString(line) {
						r.warnf(filepath.ToSlash(rel), "possible %s at line %d — secrets must not live in source", p.name, ln)
					}
				}
			}
			return nil
		})
	}
}
