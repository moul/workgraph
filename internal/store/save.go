package store

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/moul/workgraph/internal/frontmatter"
	"github.com/moul/workgraph/internal/id"
	"github.com/moul/workgraph/internal/model"
)

// bodied is implemented by objects that carry a Markdown body.
type bodied interface{ Body() string }

// Marshal serializes an object to canonical Markdown (frontmatter + body).
func Marshal(obj model.Object) ([]byte, error) {
	body := ""
	if b, ok := obj.(bodied); ok {
		body = b.Body()
	}
	return frontmatter.Marshal(obj, body)
}

// Save writes obj to its source path, computing a canonical path when the
// object has none yet. It returns the repo-relative path written.
func (w *Workspace) Save(obj model.Object) (string, error) {
	rel := obj.SourcePath()
	if rel == "" {
		var err error
		rel, err = w.canonicalPath(obj)
		if err != nil {
			return "", err
		}
		obj.SetSourcePath(rel)
	}
	abs := filepath.Join(w.Root, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", fmt.Errorf("store: mkdir: %w", err)
	}
	raw, err := Marshal(obj)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(abs, raw, 0o644); err != nil {
		return "", fmt.Errorf("store: write %s: %w", rel, err)
	}
	return rel, nil
}

// canonicalPath computes where a new object should live.
func (w *Workspace) canonicalPath(obj model.Object) (string, error) {
	switch o := obj.(type) {
	case *model.Project:
		dir := "projects/" + id.Slugify(o.Title)
		return dir + "/PROJECT.md", nil
	case *model.Item:
		if o.Project == "" {
			return "inbox/" + id.Filename(o.ID, o.Title), nil
		}
		dir, err := w.projectDir(o.Project)
		if err != nil {
			return "", err
		}
		return dir + "/items/" + id.Filename(o.ID, o.Title), nil
	case *model.Decision:
		dir, err := w.projectDir(o.Project)
		if err != nil {
			return "", err
		}
		return dir + "/decisions/" + id.Filename(o.ID, o.Title), nil
	case *model.Worker:
		slug := id.Slugify(o.Title)
		if slug == "" {
			slug = id.Slugify(o.ID)
		}
		return "workers/" + slug + ".md", nil
	default:
		return "", fmt.Errorf("store: cannot compute path for %T", obj)
	}
}

// projectDir returns the repo-relative project directory for a project id. When
// projectID is empty (a project-less inbox item), items live in top-level
// inbox/.
func (w *Workspace) projectDir(projectID string) (string, error) {
	if projectID == "" {
		return "", fmt.Errorf("store: empty project id")
	}
	g, err := w.Load()
	if err != nil {
		return "", err
	}
	o := g.ByID(projectID)
	if o == nil {
		return "", fmt.Errorf("store: unknown project %q", projectID)
	}
	// PROJECT.md lives at <dir>/PROJECT.md
	return filepath.ToSlash(filepath.Dir(o.SourcePath())), nil
}

// Delete removes an object's source file. Deletion is intentionally rare;
// callers should prefer cancelled/archived/superseded statuses.
func (w *Workspace) Delete(obj model.Object) error {
	if obj.SourcePath() == "" {
		return fmt.Errorf("store: object has no source path")
	}
	return os.Remove(filepath.Join(w.Root, obj.SourcePath()))
}
