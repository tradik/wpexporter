package exportcli

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tradik/wpexporter/pkg/models"
)

// TestEveryEntryIsQueryOnly pins what --skip-unaddressable-types keys on: a
// type publishes no addresses at all, not merely one entry that does not.
func TestEveryEntryIsQueryOnly(t *testing.T) {
	tests := []struct {
		name  string
		links []string
		want  bool
	}{
		{
			name:  "a plugin's data store",
			links: []string{"https://x.test/?modula-gallery=1289", "https://x.test/?modula-gallery=1876"},
			want:  true,
		},
		{
			name:  "one real permalink makes the type content",
			links: []string{"https://x.test/?cpt_services=12", "https://x.test/services/wms/"},
			want:  false,
		},
		{
			name:  "a type with proper addresses",
			links: []string{"https://x.test/services/wms/", "https://x.test/services/tms/"},
			want:  false,
		},
		{
			name:  "the site root is an address, not a missing one",
			links: []string{"https://x.test/"},
			want:  false,
		},
		{
			name:  "a missing link is not a query-string address",
			links: []string{""},
			want:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			posts := make([]models.WordPressPost, len(tc.links))
			for i, link := range tc.links {
				posts[i] = models.WordPressPost{Link: link}
			}

			assert.Equal(t, tc.want, everyEntryIsQueryOnly(posts))
		})
	}
}
