package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestQueryOnlyAddress pins the one definition of "this permalink names no
// document of its own", which both the address the export writes (#78) and
// --skip-unaddressable-types read.
func TestQueryOnlyAddress(t *testing.T) {
	tests := []struct {
		name string
		link string
		want bool
	}{
		{name: "a type with no rewrite rule", link: "https://x.test/?modula-gallery=1289", want: true},
		{name: "plain permalinks", link: "/?p=123", want: true},
		{name: "a page on plain permalinks", link: "https://x.test/?page_id=45", want: true},
		{name: "a pretty permalink", link: "https://x.test/services/wms/", want: false},
		{name: "the front page", link: "https://x.test/", want: false},
		{name: "a root-relative front page", link: "/", want: false},
		{name: "a path that also carries a query", link: "/blog/?page=2", want: false},
		{name: "no link at all", link: "", want: false},
		{name: "an unparsable link", link: "://not a url", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, QueryOnlyAddress(tc.link))
		})
	}
}
