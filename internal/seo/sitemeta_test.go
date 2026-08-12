package seo

import "testing"

const marketingHomePage = `<!DOCTYPE html>
<html><head>
  <meta name="google-site-verification" content="abc123">
  <meta name="facebook-domain-verification" content="fb456">
  <meta name="msvalidate.01" content="bing789">
  <meta name="theme-color" content="#0f172a">
  <meta property="og:site_name" content="Little Patisserie">
  <meta property="og:image" content="/wp-content/uploads/2024/05/social.jpg">
  <meta name="twitter:site" content="@patisserie">
  <meta name="description" content="not a marketing field">
  <link rel="icon" href="/favicon-32x32.png" sizes="32x32">
  <link rel="icon" href="/favicon-192x192.png" sizes="192x192">
  <link rel="apple-touch-icon" href="/apple-touch-icon.png">
</head>
<body>
  <header>
    <a href="https://www.facebook.com/littlepatisserie">fb</a>
    <a href="https://instagram.com/littlepatisserie">ig</a>
    <a href="https://facebook.com/">bare network page</a>
  </header>
  <main><a href="https://youtube.com/watch?v=xyz">a video in the body</a></main>
  <footer>
    <a href="https://www.linkedin.com/company/patisserie">li</a>
    <a href="/contact">contact</a>
  </footer>
</body></html>`

// TestExtractSiteMarketing covers issue #24: verification tokens, social defaults,
// icons and theme color are read from the home page.
func TestExtractSiteMarketing(t *testing.T) {
	m := extractSiteMarketing(marketingHomePage, "https://site.test/")

	if got := m.Verification["google-site-verification"]; got != "abc123" {
		t.Errorf("google verification = %q, want abc123", got)
	}
	if got := m.Verification["facebook-domain-verification"]; got != "fb456" {
		t.Errorf("facebook verification = %q, want fb456", got)
	}
	if got := m.Verification["msvalidate.01"]; got != "bing789" {
		t.Errorf("bing verification = %q, want bing789", got)
	}
	if _, unexpected := m.Verification["description"]; unexpected {
		t.Error("a plain description meta must not be recorded as verification")
	}

	if m.OGSiteName != "Little Patisserie" {
		t.Errorf("og:site_name = %q", m.OGSiteName)
	}
	if m.TwitterSite != "@patisserie" {
		t.Errorf("twitter:site = %q", m.TwitterSite)
	}
	if m.ThemeColor != "#0f172a" {
		t.Errorf("theme-color = %q", m.ThemeColor)
	}

	// Relative asset references resolve against the page they were found on.
	if m.OGImage != "https://site.test/wp-content/uploads/2024/05/social.jpg" {
		t.Errorf("og:image not resolved: %q", m.OGImage)
	}
	if m.AppleTouchIcon != "https://site.test/apple-touch-icon.png" {
		t.Errorf("apple-touch-icon = %q", m.AppleTouchIcon)
	}
	// The largest declared icon wins.
	if m.Favicon != "https://site.test/favicon-192x192.png" {
		t.Errorf("favicon = %q, want the 192x192 one", m.Favicon)
	}
}

// TestExtractSocialProfiles confirms only header/footer profile links are kept.
func TestExtractSocialProfiles(t *testing.T) {
	profiles := extractSocialProfiles(marketingHomePage)

	if got := profiles["facebook"]; got != "https://www.facebook.com/littlepatisserie" {
		t.Errorf("facebook = %q", got)
	}
	if got := profiles["instagram"]; got != "https://instagram.com/littlepatisserie" {
		t.Errorf("instagram = %q", got)
	}
	if got := profiles["linkedin"]; got != "https://www.linkedin.com/company/patisserie" {
		t.Errorf("linkedin = %q", got)
	}
	if _, found := profiles["youtube"]; found {
		t.Error("a link in the body is not a site profile")
	}
}

// TestSocialNetworkRejectsBareHomePage confirms a network's own home page is not
// mistaken for the site's profile.
func TestSocialNetworkRejectsBareHomePage(t *testing.T) {
	if _, ok := socialNetwork("https://facebook.com/"); ok {
		t.Error("bare network home page should not count as a profile")
	}
	if network, ok := socialNetwork("https://x.com/tradik"); !ok || network != "x" {
		t.Errorf("x.com profile = (%q, %v), want (x, true)", network, ok)
	}
}

// TestExtractSiteMarketingEmpty confirms a page with nothing to find reports empty,
// so the export omits the object rather than emitting empty fields.
func TestExtractSiteMarketingEmpty(t *testing.T) {
	m := extractSiteMarketing(`<html><head><title>x</title></head><body></body></html>`, "https://site.test/")
	if !m.IsEmpty() {
		t.Errorf("expected empty marketing, got %+v", m)
	}
}
