package validate

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/moul/workgraph/internal/redact"
	"github.com/moul/workgraph/internal/store"
)

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
				for _, name := range redact.Matches(sc.Text()) {
					r.warnf(filepath.ToSlash(rel), "possible %s at line %d — secrets must not live in source", name, ln)
				}
			}
			return nil
		})
	}
}
