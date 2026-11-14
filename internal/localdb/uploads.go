package localdb

import (
	"path/filepath"
	"strconv"
	"strings"
)

// Uploads says where the media library lives: the address its files are
// published under, and the directory they are on disk.
type Uploads struct {
	BaseURL string
	Dir     string
}

// Uploads works the media library's location out as WordPress does:
//
//	address: upload_url_path, else {siteurl}/{UPLOADS}, else {siteurl}/wp-content/uploads
//	disk:    upload_path,     else ABSPATH/{UPLOADS}, else WP_CONTENT_DIR/uploads,
//	         else ABSPATH/wp-content/uploads
//
// with /sites/{id} appended for a multisite network's other sites.
func (s *Source) Uploads() (Uploads, error) {
	if _, err := s.option("home"); err != nil {
		return Uploads{}, err
	}
	site, _ := s.siteURL()
	o, abs := s.options, s.settings.ABSPath

	u := Uploads{BaseURL: strings.TrimRight(o["upload_url_path"], "/")}
	switch {
	case u.BaseURL != "":
	case s.settings.Uploads != "":
		u.BaseURL = site + "/" + strings.Trim(s.settings.Uploads, "/")
	default:
		u.BaseURL = site + "/wp-content/uploads"
	}

	switch path := o["upload_path"]; {
	case path != "" && filepath.IsAbs(path):
		u.Dir = path
	case path != "":
		u.Dir = filepath.Join(abs, path)
	case s.settings.Uploads != "":
		u.Dir = filepath.Join(abs, s.settings.Uploads)
	case s.settings.ContentDir != "":
		u.Dir = filepath.Join(s.settings.ContentDir, "uploads")
	default:
		u.Dir = filepath.Join(abs, "wp-content", "uploads")
	}

	if s.opts.BlogID > 1 {
		site := "sites/" + strconv.Itoa(s.opts.BlogID)
		u.BaseURL += "/" + site
		u.Dir = filepath.Join(u.Dir, filepath.FromSlash(site))
	}
	return u, nil
}
