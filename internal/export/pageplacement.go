package export

// Where a page document lands, and what happens when two of them want the same
// file (#38).
//
// WordPress page URLs are hierarchical: /zerowisko/znaczenie/ and /znaczenie/
// are different pages that share a slug. Writing both as pages/<slug>.md let
// the second overwrite the first — 13 of 124 pages lost on one real site, with
// nothing in the output saying so. Documents are placed by their published URL
// instead, and a name still claimed twice is disambiguated and reported rather
// than resolved by whoever writes last.

import (
	"fmt"
	"path/filepath"
	"strings"
)

// pagePlacement hands out file paths and remembers what it has given away.
type pagePlacement struct {
	claimed    map[string]int
	collisions []string
	written    int
}

func newPagePlacement() *pagePlacement {
	return &pagePlacement{claimed: make(map[string]int)}
}

// claim returns the file name to write in dir for the document with this ID.
//
// A free path is granted as asked. A path another document already holds is
// granted with the WordPress ID appended — deterministic, so two runs of the
// same export produce the same tree — and the substitution is recorded, because
// a page that quietly becomes a different page is the bug this prevents.
func (p *pagePlacement) claim(dir, filename string, id int) string {
	key := filepath.Join(dir, filename)

	owner, taken := p.claimed[key]
	if !taken {
		p.claimed[key] = id
		return filename
	}

	// The same document offered twice keeps its file: rewriting it is not a
	// collision, and appending an ID would leave a stale copy behind.
	if owner == id {
		return filename
	}

	unique := fmt.Sprintf("%s-%d.md", strings.TrimSuffix(filename, ".md"), id)
	p.claimed[filepath.Join(dir, unique)] = id
	p.collisions = append(p.collisions,
		fmt.Sprintf("%s wanted by ids %d and %d — the second was written as %s", key, owner, id, unique))

	return unique
}

// recordWrite counts a document that reached disk, so the export can state
// pages written against pages fetched instead of assuming the two agree.
func (p *pagePlacement) recordWrite() {
	p.written++
}

// report describes the collisions for the console and metadata.json, or nil
// when every document got the name it asked for.
func (p *pagePlacement) report() []string {
	if len(p.collisions) == 0 {
		return nil
	}

	return p.collisions
}
