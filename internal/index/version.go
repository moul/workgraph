package index

import (
	"crypto/sha1"
	"fmt"
)

// BlobVersion computes the Git blob object id for content and returns it as
// "blob:<hex>". This matches `git hash-object` so an object's version is
// verifiable with plain git, and it is stable regardless of where the file
// lives. Object versions are derived, never stored in frontmatter.
func BlobVersion(content []byte) string {
	h := sha1.New()
	fmt.Fprintf(h, "blob %d\x00", len(content))
	h.Write(content)
	return "blob:" + fmt.Sprintf("%x", h.Sum(nil))
}
