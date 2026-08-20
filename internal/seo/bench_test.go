package seo

// Benchmarks for the crawl, which runs once per fetched page. Both of these
// used to compile their regular expressions on every call.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

// renderedPage is the shape a WordPress theme actually serves: a head full of
// meta tags, and a body of builder wrappers around the content.
func renderedPage() string {
	var b strings.Builder

	b.WriteString(`<!DOCTYPE html><html lang="pl"><head><title>Strona — Serwis</title>`)
	for _, name := range []string{"description", "keywords", "robots", "author", "generator"} {
		fmt.Fprintf(&b, `<meta name="%s" content="Wartosc dla %s">`, name, name)
	}
	for _, prop := range []string{"og:title", "og:description", "og:image", "og:type", "og:url"} {
		fmt.Fprintf(&b, `<meta property="%s" content="Wartosc dla %s">`, prop, prop)
	}
	b.WriteString(`<link rel="canonical" href="https://x.test/strona/">`)
	for _, lang := range []string{"pl", "en", "de"} {
		fmt.Fprintf(&b, `<link rel="alternate" hreflang="%s" href="https://x.test/%s/">`, lang, lang)
	}
	b.WriteString(`</head><body class="home page page-id-12"><header>Menu</header><main id="content">`)

	for i := 0; i < 30; i++ {
		fmt.Fprintf(&b, `<div class="kc-elm kc_row"><div class="kc_column">`+
			`<h2>Sekcja %d</h2><p>Tresc akapitu, ktora ma dosc slow by cos znaczyc na stronie.</p>`+
			`</div></div>`, i)
	}

	return b.String() + `</main><footer>Stopka</footer></body></html>`
}

func BenchmarkPopulateSEO(b *testing.B) {
	page := renderedPage()
	crawler := NewCrawler(&config.Config{Timeout: 5, Concurrent: 1})
	b.ReportAllocs()
	b.SetBytes(int64(len(page)))

	for b.Loop() {
		var seo models.SEOData
		crawler.populateSEO(&seo, page)
	}
}

func BenchmarkExtractMainContent(b *testing.B) {
	page := renderedPage()
	crawler := NewCrawler(&config.Config{Timeout: 5, Concurrent: 1})
	b.ReportAllocs()
	b.SetBytes(int64(len(page)))

	for b.Loop() {
		_ = crawler.extractMainContent(page)
	}
}
