package export

// Pages the theme builds and the API does not serve (#46).
//
// A front page built with a theme's own section builder — GeneratePress
// Sections, Elementor, Avada's builder, Smart Slider — stores its sections in
// post meta and assembles them at render time. `/wp/v2/pages/<id>` therefore
// answers with `content.rendered` that is empty or nearly so, and the export is
// correct and useless at the same time: it faithfully records an empty front
// page. The migrated home page arrives as chrome with nothing in it, while the
// source shows a hero, a slider and three sections of copy. Two of six sites in
// one batch hit this.
//
// Silence is the worst part — the migration reports success and the operator
// finds out by looking at the built site. So the export says it: which page,
// and that its body was never in the API to begin with. The front page is named
// first, because it is the one the whole site opens with.

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/tradik/wpexporter/pkg/models"
)

// emptyPageTextBudget is how little visible text makes a page "empty enough to
// be worth reporting".
//
// Not zero: a builder page often keeps a heading and a sentence in the editor
// while everything else lives in sections, and that is exactly the case to
// report — the export carries a heading where the site shows a hero, a slider
// and three sections.
const emptyPageTextBudget = 120

// reportEmptyPages names every page whose body the API did not serve, and
// records the same lines in metadata.json.
//
// A page already reported as a post loop (#41) is left alone: it has been
// explained once, and by the more specific of the two explanations.
func (e *Exporter) reportEmptyPages(data *models.ExportData) {
	home := normalizeSiteURL(data.Site.HomeURL, data.Site.URL)

	var notices []string

	for i := range data.Pages {
		page := &data.Pages[i]

		if postLoopHint(page.Content.Rendered) != "" {
			continue
		}
		if len(visibleText(page.Content.Rendered)) > emptyPageTextBudget {
			continue
		}

		notices = append(notices, emptyPageNotice(*page, isFrontPage(*page, home)))
	}

	if len(notices) == 0 {
		return
	}

	data.Stats.EmptyPages = notices

	if !e.config.Quiet {
		for _, notice := range notices {
			fmt.Println(notice)
		}
	}
}

// emptyPageNotice is the line an operator reads. It says which page, what the
// export contains, and where the missing part actually lives — a builder's
// sections are not something a re-run will fetch.
func emptyPageNotice(page models.WordPressPost, front bool) string {
	subject := "page " + pageAddress(page)
	if front {
		subject = "front page " + pageAddress(page)
	}

	return "Warning: " + subject + " exports with almost no content. The REST API serves " +
		"what is stored, and a page built from theme sections or a page builder stores its " +
		"body elsewhere — try --assisted-crawl --crawl-content to take the rendered page instead."
}

// pageAddress is the clearest way to name a page in a report: its address if it
// has one, its slug otherwise.
func pageAddress(page models.WordPressPost) string {
	if page.Link != "" {
		return page.Link
	}
	if page.Slug != "" {
		return "/" + page.Slug + "/"
	}

	return fmt.Sprintf("#%d", page.ID)
}

// isFrontPage reports whether a page is the one the site opens with. The API
// does not say which it is in a public read, so it is recognized by address.
func isFrontPage(page models.WordPressPost, home string) bool {
	if home == "" || page.Link == "" {
		return false
	}

	return normalizeSiteURL(page.Link, "") == home
}

// normalizeSiteURL reduces an address to a comparable path, preferring the
// first value that is one.
func normalizeSiteURL(preferred, fallback string) string {
	for _, raw := range []string{preferred, fallback} {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}

		parsed, err := url.Parse(raw)
		if err != nil {
			continue
		}

		path := strings.TrimSuffix(parsed.Path, "/")

		return strings.ToLower(parsed.Host + path)
	}

	return ""
}
