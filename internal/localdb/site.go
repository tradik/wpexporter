package localdb

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/tradik/wpexporter/pkg/models"
)

// siteOptions are the only rows of the options table this package reads,
// apart from the active theme's theme_mods_* row (menus.go). Never SELECT *:
// the table also holds API keys, license keys and mail credentials.
var siteOptions = []string{
	"blogname", "blogdescription", "siteurl", "home", "timezone_string", "gmt_offset",
	"date_format", "time_format", "start_of_week", "WPLANG", "show_on_front",
	"page_on_front", "page_for_posts", "permalink_structure", "category_base",
	"tag_base", "sticky_posts", "stylesheet", "upload_path", "upload_url_path",
}

// option returns one of siteOptions, loading them all on first use.
func (s *Source) option(name string) (string, error) {
	if s.options == nil {
		opts := map[string]string{}
		err := s.q.each(func(r *sql.Rows) error {
			var k string
			var v sql.NullString
			if err := r.Scan(&k, &v); err != nil {
				return err
			}
			opts[k] = v.String
			return nil
		}, "SELECT option_name, option_value FROM {options} WHERE option_name IN (?)", list(siteOptions))
		if err != nil {
			return "", s.fail(fmt.Errorf("reading site options: %w", err))
		}
		s.options = opts
	}
	return s.options[name], nil
}

// home is the site's address: WP_HOME when wp-config.php defines it, else the
// home option. Without a trailing slash.
func (s *Source) home() (string, error) {
	return s.constOrOption(s.settings.Home, "home")
}

// siteURL is where WordPress itself lives: WP_SITEURL, else the siteurl option.
func (s *Source) siteURL() (string, error) {
	return s.constOrOption(s.settings.SiteURL, "siteurl")
}

// constOrOption prefers a wp-config.php constant over the stored option.
func (s *Source) constOrOption(constant, name string) (string, error) {
	if constant != "" {
		return strings.TrimRight(constant, "/"), nil
	}
	v, err := s.option(name)
	return strings.TrimRight(v, "/"), err
}

// Home returns the site's address, for a run given --local without --url.
func (s *Source) Home() (string, error) { return s.home() }

// GetSiteInfo describes the site from its options. AdminEmail stays empty: the
// public REST API does not publish it either.
func (s *Source) GetSiteInfo() (*models.SiteInfo, error) {
	if _, err := s.option("home"); err != nil {
		return nil, err
	}
	o := s.options
	home, _ := s.home()
	site, _ := s.siteURL()

	info := &models.SiteInfo{
		Name: o["blogname"], Description: o["blogdescription"], URL: site, HomeURL: home,
		Timezone: timezone(o["timezone_string"], o["gmt_offset"]), DateFormat: o["date_format"],
		TimeFormat: o["time_format"], Language: o["WPLANG"], ShowOnFront: o["show_on_front"],
	}
	info.StartOfWeek, _ = strconv.Atoi(o["start_of_week"])
	if info.Language == "" {
		info.Language = "en_US"
	}

	pages, err := s.sitePages(o["page_on_front"], o["page_for_posts"])
	if err != nil {
		return nil, err
	}
	info.FrontPage = sitePage(pages, o["page_on_front"])
	info.PostsPage = sitePage(pages, o["page_for_posts"])
	return info, nil
}

// sitePage describes the page with the given ID, or nil when it is not one of
// the published pages.
func sitePage(pages []models.WordPressPost, id string) *models.SitePage {
	n, _ := strconv.Atoi(id)
	for _, p := range pages {
		if n > 0 && p.ID == n {
			return &models.SitePage{ID: p.ID, Slug: p.Slug, Link: p.Link}
		}
	}
	return nil
}

// timezone is the site's zone name, or its fixed offset written as WordPress
// writes it ("UTC+2", "UTC-5:30") when no zone was chosen.
func timezone(name, offset string) string {
	if name != "" {
		return name
	}
	f, err := strconv.ParseFloat(offset, 64)
	if err != nil || f == 0 {
		return "UTC"
	}
	sign := "+"
	if f < 0 {
		sign, f = "-", -f
	}
	hours, minutes := int(f), int((f-float64(int(f)))*60+0.5)
	if minutes == 0 {
		return fmt.Sprintf("UTC%s%d", sign, hours)
	}
	return fmt.Sprintf("UTC%s%d:%02d", sign, hours, minutes)
}
