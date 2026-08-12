package seo

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/tradik/wpexporter/pkg/models"
)

var (
	// linkTagPattern matches a complete <link> element.
	linkTagPattern = regexp.MustCompile(`(?is)<link\b[^>]*>`)
	// anchorTagPattern matches an <a> element's opening tag.
	anchorTagPattern = regexp.MustCompile(`(?is)<a\b[^>]*>`)
	// headerFooterPattern isolates the regions where a theme puts its social links.
	headerFooterPattern = regexp.MustCompile(`(?is)<(header|footer)\b[^>]*>.*?</(?:header|footer)\s*>`)
)

// verificationKeys are the ownership-proof meta names worth carrying to the target
// platform. Anything else stays in the generic meta map.
var verificationKeys = map[string]bool{
	"google-site-verification":         true,
	"facebook-domain-verification":     true,
	"msvalidate.01":                    true,
	"yandex-verification":              true,
	"p:domain_verify":                  true,
	"norton-safeweb-site-verification": true,
}

// socialHosts maps a profile host to the network name it is recorded under. The
// longest match wins, so "business.facebook.com" is still facebook.
var socialHosts = []struct {
	host    string
	network string
}{
	{"facebook.com", "facebook"},
	{"instagram.com", "instagram"},
	{"linkedin.com", "linkedin"},
	{"youtube.com", "youtube"},
	{"youtu.be", "youtube"},
	{"twitter.com", "x"},
	{"x.com", "x"},
	{"tiktok.com", "tiktok"},
	{"pinterest.com", "pinterest"},
}

// SiteMarketing fetches the site's home page once and extracts the marketing and
// brand wiring a migration has to re-create: verification tokens, social profiles
// and defaults, favicon/apple-touch-icon/logo and theme color.
//
// It returns nil when the page cannot be read or nothing was found — the export
// records what exists rather than inventing placeholder values.
func (c *Crawler) SiteMarketing(homeURL string) *models.SiteMarketing {
	if homeURL == "" {
		return nil
	}

	html := c.fetchHTML(homeURL)
	if html == "" {
		return nil
	}

	// The home page carries tracking snippets too, and it may be the only page that
	// does (a GTM container loaded site-wide still has to be seen somewhere).
	c.recordAnalytics(extractAnalytics(html))

	marketing := extractSiteMarketing(html, homeURL)
	if marketing.IsEmpty() {
		return nil
	}

	return &marketing
}

// extractSiteMarketing pulls the site-level fields out of one page's HTML.
// baseURL resolves relative asset paths so a consumer gets a usable address.
func extractSiteMarketing(html, baseURL string) models.SiteMarketing {
	marketing := models.SiteMarketing{}

	base, _ := url.Parse(baseURL)

	for _, tag := range parseMetaTags(html) {
		switch {
		case verificationKeys[tag.key]:
			if marketing.Verification == nil {
				marketing.Verification = make(map[string]string)
			}
			if _, seen := marketing.Verification[tag.key]; !seen {
				marketing.Verification[tag.key] = tag.content
			}
		case tag.key == "og:site_name" && marketing.OGSiteName == "":
			marketing.OGSiteName = tag.content
		case tag.key == "og:image" && marketing.OGImage == "":
			marketing.OGImage = absoluteURL(base, tag.content)
		case tag.key == "twitter:site" && marketing.TwitterSite == "":
			marketing.TwitterSite = tag.content
		case tag.key == "theme-color" && marketing.ThemeColor == "":
			marketing.ThemeColor = tag.content
		}
	}

	extractIcons(html, base, &marketing)
	marketing.SocialProfiles = extractSocialProfiles(html)

	// A site that declares no explicit logo still usually has an og:image standing
	// in for one; recording it as the logo would be a guess, so it is left alone.
	return marketing
}

// extractIcons reads the icon links from the document head. The largest declared
// icon wins for the favicon, since a target platform generally wants the biggest
// source it can downscale.
func extractIcons(html string, base *url.URL, marketing *models.SiteMarketing) {
	bestFavicon := 0

	for _, element := range linkTagPattern.FindAllString(html, -1) {
		attrs := parseTagAttributes(element)
		href := strings.TrimSpace(attrs["href"])
		if href == "" {
			continue
		}

		rel := strings.ToLower(strings.TrimSpace(attrs["rel"]))
		switch {
		case rel == "apple-touch-icon" || rel == "apple-touch-icon-precomposed":
			if marketing.AppleTouchIcon == "" {
				marketing.AppleTouchIcon = absoluteURL(base, href)
			}
		case strings.Contains(rel, "icon") && !strings.Contains(rel, "mask"):
			if size := largestIconSize(attrs["sizes"]); marketing.Favicon == "" || size > bestFavicon {
				marketing.Favicon = absoluteURL(base, href)
				bestFavicon = size
			}
		case rel == "logo":
			if marketing.Logo == "" {
				marketing.Logo = absoluteURL(base, href)
			}
		}
	}
}

// largestIconSize reads the leading dimension of a sizes attribute ("32x32",
// "192x192 512x512"), returning 0 when it is absent or "any".
func largestIconSize(sizes string) int {
	best := 0

	for _, field := range strings.Fields(strings.ToLower(sizes)) {
		width, _, ok := strings.Cut(field, "x")
		if !ok {
			continue
		}

		n := 0
		for _, r := range width {
			if r < '0' || r > '9' {
				n = 0
				break
			}
			n = n*10 + int(r-'0')
		}

		if n > best {
			best = n
		}
	}

	return best
}

// extractSocialProfiles collects profile links from the header and footer, where
// themes put them. Restricting the search to those regions avoids picking up a
// share button or an outbound link from the body.
func extractSocialProfiles(html string) map[string]string {
	regions := headerFooterPattern.FindAllString(html, -1)
	if len(regions) == 0 {
		return nil
	}

	profiles := make(map[string]string)

	for _, region := range regions {
		for _, element := range anchorTagPattern.FindAllString(region, -1) {
			href := strings.TrimSpace(parseTagAttributes(element)["href"])
			if href == "" {
				continue
			}

			network, ok := socialNetwork(href)
			if !ok {
				continue
			}
			if _, seen := profiles[network]; !seen {
				profiles[network] = href
			}
		}
	}

	if len(profiles) == 0 {
		return nil
	}

	return profiles
}

// socialNetwork maps a URL to the network it belongs to. A bare network home page
// (no profile path) is rejected: it is chrome, not the site's profile.
func socialNetwork(rawURL string) (string, bool) {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		return "", false
	}

	host := strings.ToLower(strings.TrimPrefix(parsed.Host, "www."))

	for _, candidate := range socialHosts {
		if host != candidate.host && !strings.HasSuffix(host, "."+candidate.host) {
			continue
		}
		if strings.Trim(parsed.Path, "/") == "" {
			return "", false // network home page, not a profile
		}

		return candidate.network, true
	}

	return "", false
}

// absoluteURL resolves a possibly-relative asset reference against the page it was
// found on, so the exported value is usable without knowing the source host.
func absoluteURL(base *url.URL, ref string) string {
	ref = strings.TrimSpace(ref)
	if base == nil || ref == "" {
		return ref
	}

	parsed, err := url.Parse(ref)
	if err != nil {
		return ref
	}

	return base.ResolveReference(parsed).String()
}
