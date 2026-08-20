package export

// Benchmarks for the document conversion, which runs once per exported page and
// is the second-largest cost after decoding. The preservation pass is measured
// on its own because it once compiled a regular expression per element.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tradik/wpexporter/internal/config"
)

// wpBody is a page body of the shape WordPress actually serves: block wrappers,
// headings with classes, paragraphs, a list, links and an image.
func wpBody() string {
	var b strings.Builder
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&b, `<div class="wp-block-group sc_section trx_addons_inline_%d">`+
			`<h2 class="wp-block-heading sc_item_title">Sekcja %d</h2>`+
			`<p class="wp-block-paragraph">Tekst akapitu, ktory ma dosc slow by cos znaczyc. `+
			`<strong>Wyroznienie</strong> oraz <em>kursywa</em> i <a href="/x/">odnosnik</a>.</p>`+
			`<ul class="wp-block-list"><li>Pierwszy</li><li>Drugi</li><li>Trzeci</li></ul>`+
			`<img src="/wp-content/uploads/x%d.jpg" class="wp-image-%d" alt="Obraz">`+
			`</div>`, i, i, i, i)
	}

	return b.String()
}

func BenchmarkHTMLToMarkdown(b *testing.B) {
	body := wpBody()
	b.ReportAllocs()
	b.SetBytes(int64(len(body)))

	for b.Loop() {
		_ = htmlToMarkdown(body)
	}
}

func BenchmarkHTMLToMarkdownKeepingHeadings(b *testing.B) {
	body := wpBody()
	rules := NewExporter(&config.Config{}).preserveRules()
	b.ReportAllocs()
	b.SetBytes(int64(len(body)))

	for b.Loop() {
		_ = htmlToMarkdownKeeping(body, rules)
	}
}

func BenchmarkPreserveElements(b *testing.B) {
	body := wpBody()
	rules := preserveRules{classes: []string{"trx_addons_inline_*"}, styledHeadings: true}
	b.ReportAllocs()

	for b.Loop() {
		_, _ = preserveElements(body, rules)
	}
}
