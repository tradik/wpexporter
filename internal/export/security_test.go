package export

import (
	"strings"
	"testing"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

// TestCSVSafe verifies CSV formula-injection neutralization (INT-001).
func TestCSVSafe(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"Normal title", "Normal title"},
		{"=HYPERLINK(\"http://evil\")", "'=HYPERLINK(\"http://evil\")"},
		{"+1+1", "'+1+1"},
		{"-2+3", "'-2+3"},
		{"@SUM(A1:A2)", "'@SUM(A1:A2)"},
		{"\tTabbed", "'\tTabbed"},
		{"\rCarriage", "'\rCarriage"},
		{"=cmd|'/c calc'!A1", "'=cmd|'/c calc'!A1"},
	}
	for _, tt := range tests {
		if got := csvSafe(tt.in); got != tt.want {
			t.Errorf("csvSafe(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestCSVSafeRow verifies each field of a row is neutralized.
func TestCSVSafeRow(t *testing.T) {
	in := []string{"safe", "=danger", "@also", "plain"}
	got := csvSafeRow(in)
	want := []string{"safe", "'=danger", "'@also", "plain"}
	if len(got) != len(want) {
		t.Fatalf("csvSafeRow() len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("csvSafeRow()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	// Ensure the input slice is not mutated in place.
	if in[1] != "=danger" {
		t.Errorf("csvSafeRow() mutated input: in[1] = %q", in[1])
	}
}

// TestSafeHref verifies href sanitization for FE-001.
func TestSafeHref(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"https://example.com/x", "https://example.com/x"},
		{"http://example.com", "http://example.com"},
		{"mailto:a@b.com", "mailto:a@b.com"},
		{"javascript:alert(1)", "#"},
		{"vbscript:msgbox(1)", "#"},
		{"data:text/html,<script>", "#"},
	}
	for _, tt := range tests {
		if got := safeHref(tt.in); got != tt.want {
			t.Errorf("safeHref(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}

	// A malformed URL used for attribute breakout must never contain raw
	// HTML-significant characters in the result.
	got := safeHref(`"><img src=x onerror=alert(1)>`)
	for _, bad := range []string{`"`, "<", ">"} {
		if strings.Contains(got, bad) {
			t.Errorf("safeHref() breakout escape leaked %q in %q", bad, got)
		}
	}
}

// TestGenerateMetadataHTMLEscapesInjection verifies that attacker-controlled
// fields cannot break out of the generated HTML (FE-001).
func TestGenerateMetadataHTMLEscapesInjection(t *testing.T) {
	s := NewShopifyExporter(&config.Config{})
	post := models.WordPressPost{
		ID:     1,
		Slug:   "ok-slug",
		Status: "publish",
		Type:   "post",
		Link:   `"><img src=x onerror=alert(1)>`,
		SEO: models.SEOData{
			Lang: `en"><script>alert(1)</script>`,
			Hreflangs: []models.HreflangLink{
				{Lang: "en", Href: `javascript:alert(1)`},
			},
		},
	}

	out := s.generateMetadataHTML(post, "Evil<script>", "Cat&<b>", "Tag\"1")

	if strings.Contains(out, "<img src=x onerror=alert(1)>") {
		t.Error("generateMetadataHTML() did not escape injected <img> from post.Link")
	}
	if strings.Contains(out, "<script>alert(1)</script>") {
		t.Error("generateMetadataHTML() did not escape injected <script> from SEO.Lang")
	}
	if strings.Contains(out, `href="javascript:`) {
		t.Error("generateMetadataHTML() allowed a javascript: href in hreflang")
	}
	if strings.Contains(out, "Evil<script>") {
		t.Error("generateMetadataHTML() did not escape author name")
	}
}
