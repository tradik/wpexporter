package localdb

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUploads(t *testing.T) {
	cases := []struct {
		name       string
		options    map[string]string
		uploads    string
		contentDir string
		blogID     int
		wantURL    string
		wantDir    string
	}{
		{name: "defaults", wantURL: "https://example.com/wp-content/uploads", wantDir: "/var/www/html/wp-content/uploads"},
		{name: "options", options: map[string]string{"upload_url_path": "https://cdn.example.com/files/", "upload_path": "/srv/files"},
			wantURL: "https://cdn.example.com/files", wantDir: "/srv/files"},
		{name: "relative upload_path", options: map[string]string{"upload_path": "files"},
			wantURL: "https://example.com/wp-content/uploads", wantDir: "/var/www/html/files"},
		{name: "UPLOADS", uploads: "/media/", wantURL: "https://example.com/media", wantDir: "/var/www/html/media"},
		{name: "WP_CONTENT_DIR", contentDir: "/srv/content",
			wantURL: "https://example.com/wp-content/uploads", wantDir: "/srv/content/uploads"},
		{name: "multisite", blogID: 3,
			wantURL: "https://example.com/wp-content/uploads/sites/3", wantDir: "/var/www/html/wp-content/uploads/sites/3"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := settings()
			s.Uploads, s.ContentDir = tc.uploads, tc.contentDir
			opts := map[string]string{"home": "https://example.com", "siteurl": "https://example.com"}
			for k, v := range tc.options {
				opts[k] = v
			}
			src := &Source{settings: s, opts: Options{BlogID: tc.blogID}, options: opts}
			up, err := src.Uploads()
			require.NoError(t, err)
			assert.Equal(t, Uploads{BaseURL: tc.wantURL, Dir: tc.wantDir}, up)
		})
	}
}
