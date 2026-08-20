package flathtml

// Benchmark for the --flat-html conversion, whose rules are compiled once per
// converter and whose preservation pass was compiled once per document.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tradik/wpexporter/pkg/models"
)

func builderBody() string {
	var b strings.Builder
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&b, `<div class="brxe-heading"><h2>Naglowek %d</h2></div>`+
			`<div class="brxe-text"><p>Tresc akapitu z <strong>wyroznieniem</strong> `+
			`oraz <a href="/x/">odnosnikiem</a>.</p></div>`+
			`<ul><li>Jeden</li><li>Dwa</li></ul>`, i)
	}

	return b.String()
}

func BenchmarkConvertPosts(b *testing.B) {
	body := builderBody()
	converter := NewConverterWithOptions(nil, &PreserveOptions{Classes: []string{"klaviyo-form-*"}})

	posts := make([]models.WordPressPost, 5)
	for i := range posts {
		posts[i].Content.Rendered = body
	}

	b.ReportAllocs()
	b.SetBytes(int64(len(body) * len(posts)))

	for b.Loop() {
		batch := make([]models.WordPressPost, len(posts))
		copy(batch, posts)
		_ = converter.ConvertPosts(batch)
	}
}
