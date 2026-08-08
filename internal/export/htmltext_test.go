package export

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecodeTypographicEntities(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"en dash", "2010 &#8211; 2011", "2010 – 2011"},
		{"curly quotes", "&#8220;quoted&#8221;", "“quoted”"},
		{"ellipsis", "more&hellip;", "more…"},
		{"arrow", "next &rarr;", "next →"},
		{"nbsp", "a&nbsp;b", "a b"},
		{"lt stays encoded", "&lt;script&gt;", "&lt;script&gt;"},
		{"amp stays encoded", "a &amp; b", "a &amp; b"},
		{"quot stays encoded", "&quot;x&quot;", "&quot;x&quot;"},
		{"numeric lt stays encoded", "&#60;b&#62;", "&#60;b&#62;"},
		{"unknown entity untouched", "&notanentity;", "&notanentity;"},
		{"no entities", "plain text", "plain text"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, decodeTypographicEntities(tt.in))
		})
	}
}

// TestDecodeTypographicEntitiesDoesNotResurrectMarkup pins the security-relevant
// half: escaped markup must stay escaped.
func TestDecodeTypographicEntitiesDoesNotResurrectMarkup(t *testing.T) {
	in := `&lt;script&gt;alert(&#39;x&#39;)&lt;/script&gt; &#8212; note`

	got := decodeTypographicEntities(in)

	assert.NotContains(t, got, "<script>")
	assert.Contains(t, got, "—", "typographic entities should still decode")
}

func TestPlainText(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"tags stripped", "<p>Hello <b>world</b></p>", "Hello world"},
		{"entities decoded", "<p>a &amp; b</p>", "a & b"},
		{"whitespace collapsed", "<p>a\n\n   b</p>", "a b"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, plainText(tt.in))
		})
	}
}

func TestPlainTextExcerpt(t *testing.T) {
	// The shape WordPress actually generates, as reported in #11.
	in := `<p>Some swimming text&hellip; ` +
		`<a href="https://hawanas.com/2010/07/21/389/" class="more-link">` +
		`Continue reading <span class="meta-nav">&rarr;</span></a></p>`

	assert.Equal(t, "Some swimming text…", plainTextExcerpt(in))
}

func TestStripReadMoreAnchor(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "more-link class",
			in:   `Text&hellip; <a href="/x" class="more-link">Whatever</a>`,
			want: "Text&hellip;",
		},
		{
			name: "continue reading text",
			in:   `Text&hellip; <a href="/x">Continue reading &rarr;</a>`,
			want: "Text&hellip;",
		},
		{
			name: "read more text",
			in:   `Text <a href="/x">Read more</a>`,
			want: "Text",
		},
		{
			name: "localized phrase",
			in:   `Tekst <a href="/x">Czytaj dalej</a>`,
			want: "Tekst",
		},
		{
			name: "genuine trailing link kept",
			in:   `See <a href="https://example.org/">the spec</a>`,
			want: `See <a href="https://example.org/">the spec</a>`,
		},
		{
			name: "no anchor",
			in:   "Just text",
			want: "Just text",
		},
		{
			name: "anchor not at end kept",
			in:   `<a href="/x">Read more</a> then text`,
			want: `<a href="/x">Read more</a> then text`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, stripReadMoreAnchor(tt.in))
		})
	}
}

func TestCleanImages(t *testing.T) {
	altByURL := map[string]string{
		"/media/images/391_fran1.jpg": "Swimmer mid-stroke",
	}

	tests := []struct {
		name        string
		in          string
		wantContain []string
		wantMissing []string
	}{
		{
			name: "alt filled from media library",
			in:   `<img src="/media/images/391_fran1.jpg">`,
			wantContain: []string{
				`src="/media/images/391_fran1.jpg"`,
				`alt="Swimmer mid-stroke"`,
			},
		},
		{
			name:        "empty alt filled",
			in:          `<img src="/media/images/391_fran1.jpg" alt="">`,
			wantContain: []string{`alt="Swimmer mid-stroke"`},
		},
		{
			name:        "existing alt preserved",
			in:          `<img src="/media/images/391_fran1.jpg" alt="Author's own">`,
			wantContain: []string{`alt="Author&#39;s own"`},
			wantMissing: []string{"Swimmer mid-stroke"},
		},
		{
			name:        "wordpress classes dropped",
			in:          `<img src="/media/images/391_fran1.jpg" class="wp-image-391 size-full alignnone">`,
			wantMissing: []string{"class="},
		},
		{
			name:        "authored classes kept",
			in:          `<img src="/media/images/391_fran1.jpg" class="wp-image-391 my-hero">`,
			wantContain: []string{`class="my-hero"`},
		},
		{
			name:        "filename-repeating title dropped",
			in:          `<img src="/media/images/391_fran1.jpg" title="fran1">`,
			wantMissing: []string{"title="},
		},
		{
			name:        "meaningful title kept",
			in:          `<img src="/media/images/391_fran1.jpg" title="Poolside, 2010">`,
			wantContain: []string{`title="Poolside, 2010"`},
		},
		{
			name:        "browser hints dropped",
			in:          `<img src="/media/images/391_fran1.jpg" loading="lazy" decoding="async" sizes="(max-width: 400px) 100vw">`,
			wantMissing: []string{"loading=", "decoding=", "sizes="},
		},
		{
			name:        "unknown image left without alt",
			in:          `<img src="/media/images/999_other.jpg">`,
			wantMissing: []string{"alt="},
		},
		{
			name:        "non-image markup untouched",
			in:          `<p>text</p>`,
			wantContain: []string{"<p>text</p>"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanImages(tt.in, altByURL)

			for _, want := range tt.wantContain {
				assert.Contains(t, got, want)
			}
			for _, missing := range tt.wantMissing {
				assert.NotContains(t, got, missing)
			}
		})
	}
}

// TestCleanImagesDoesNotDoubleEscape pins the round-trip: a value parsed out of
// the source is decoded once and re-escaped once.
func TestCleanImagesDoesNotDoubleEscape(t *testing.T) {
	in := `<img src="/media/images/1_a.jpg?w=1&amp;h=2" alt="A &amp; B">`

	got := cleanImages(in, nil)

	assert.Contains(t, got, `src="/media/images/1_a.jpg?w=1&amp;h=2"`)
	assert.Contains(t, got, `alt="A &amp; B"`)
	assert.NotContains(t, got, "&amp;amp;")
}

func TestCleanImagesSingleQuotedAttributes(t *testing.T) {
	got := cleanImages(`<img src='/media/images/391_fran1.jpg'>`, map[string]string{
		"/media/images/391_fran1.jpg": "Swimmer",
	})

	assert.Contains(t, got, `src="/media/images/391_fran1.jpg"`)
	assert.Contains(t, got, `alt="Swimmer"`)
}

func TestKeepContentClasses(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"wp-image-391 size-full", ""},
		{"wp-image-391 hero", "hero"},
		{"alignnone aligncenter", ""},
		{"attachment-large wp-post-image", ""},
		{"wp-block-image", ""},
		{"custom another", "custom another"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			assert.Equal(t, tt.want, keepContentClasses(tt.in))
		})
	}
}

func TestRepeatsFilename(t *testing.T) {
	tests := []struct {
		name  string
		title string
		src   string
		want  bool
	}{
		{"exact stem", "fran1", "/media/images/391_fran1.jpg", true},
		{"case insensitive", "FRAN1", "/media/images/391_fran1.jpg", true},
		{"no id prefix", "DSC_1739", "/uploads/DSC_1739.jpg", true},
		{"different text", "Poolside", "/media/images/391_fran1.jpg", false},
		{"empty title", "", "/media/images/391_fran1.jpg", false},
		{"empty src", "fran1", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, repeatsFilename(tt.title, tt.src))
		})
	}
}
