package export

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

// excerptEndingIn builds the shape WordPress generates: the excerpt's own <p>
// with the theme's link inside it.
func excerptEndingIn(text, phrase string) string {
	return `<p>` + text + ` <a href="https://x.test/post/">` + phrase + `</a></p>`
}

// TestReadMoreIsLearnedFromTheSite: the phrase list was seven strings in six
// European languages, and "read more" exists in every language there is. A
// theme repeats itself, so the site says what its phrase is: the trailing link
// text that ends several excerpts is the theme speaking.
func TestReadMoreIsLearnedFromTheSite(t *testing.T) {
	// Japanese, on no list anywhere.
	excerpts := []string{
		excerptEndingIn("最初の記事の要約です", "続きを読む"),
		excerptEndingIn("二番目の記事の要約です", "続きを読む"),
		excerptEndingIn("三番目の記事の要約です", "続きを読む"),
	}

	learned := newReadMoreVocabulary(excerpts, nil)

	assert.Equal(t, "最初の記事の要約です", plainTextExcerpt(excerpts[0], learned))
	assert.False(t, seededVocabulary().isReadMore("", "続きを読む"),
		"and it is not something the seed could have known")
}

// TestALinkThatEndsOneExcerptIsContent: the other side of the rule, and the
// reason it counts rather than assumes. A post whose summary ends by linking to
// the specification it discusses must keep that link.
func TestALinkThatEndsOneExcerptIsContent(t *testing.T) {
	excerpts := []string{
		excerptEndingIn("Omówienie normy", "PN-EN 1090"),
		excerptEndingIn("Drugi wpis", "Czytaj dalej"),
		excerptEndingIn("Trzeci wpis", "Czytaj dalej"),
		excerptEndingIn("Czwarty wpis", "Czytaj dalej"),
	}

	learned := newReadMoreVocabulary(excerpts, nil)

	assert.Contains(t, plainTextExcerpt(excerpts[0], learned), "PN-EN 1090")
	assert.Equal(t, "Drugi wpis", plainTextExcerpt(excerpts[1], learned))
}

// TestStructuralReadMoreNeedsNoLanguage: WordPress's own link carries marks
// that are markup rather than words, and markup is the same everywhere.
func TestStructuralReadMoreNeedsNoLanguage(t *testing.T) {
	vocabulary := seededVocabulary()

	assert.True(t, vocabulary.isReadMore(`class="more-link"`, "Διαβάστε περισσότερα"))
	assert.True(t, vocabulary.isReadMore(`rel="bookmark"`, "اقرأ المزيد"))
	assert.True(t, vocabulary.isReadMore(`class="btn read-more"`, "لا شيء"))
	assert.False(t, vocabulary.isReadMore(`class="reference"`, "Διαβάστε περισσότερα"))
}

// TestAnArrowIsNotASentence: a theme is free to write "→" and nothing else, and
// no language makes punctuation into a sentence.
func TestAnArrowIsNotASentence(t *testing.T) {
	vocabulary := seededVocabulary()

	for _, arrow := range []string{"→", "»", "…", ">>", "->", " → "} {
		assert.True(t, vocabulary.isReadMore("", arrow), "arrow %q", arrow)
	}

	assert.False(t, vocabulary.isReadMore("", "the spec"))
}

// TestALongTrailingLinkIsNotChrome: a theme's phrase is a handful of words. An
// excerpt ending in a link that is a whole clause is that post's own writing.
func TestALongTrailingLinkIsNotChrome(t *testing.T) {
	long := "przeczytaj caly artykul o normach i wymaganiach dla konstrukcji stalowych"
	excerpts := []string{
		excerptEndingIn("A", long),
		excerptEndingIn("B", long),
		excerptEndingIn("C", long),
	}

	learned := newReadMoreVocabulary(excerpts, nil)
	assert.False(t, learned.isReadMore("", long), "too long to be a theme's chrome")
}

// TestOperatorNamesThePhrase: a one-post export has nothing to learn from, and
// an operator who knows their theme can say so outright.
func TestOperatorNamesThePhrase(t *testing.T) {
	named := newReadMoreVocabulary(nil, []string{"lees verder", "  "})

	assert.True(t, named.isReadMore("", "Lees verder"))
	assert.False(t, named.isReadMore("", "read more"),
		"naming a phrase replaces the guesses rather than adding to them")
}

// TestExporterLearnsBeforeItWrites: the vocabulary has to be built from every
// document the export carries, before any of them is written.
func TestExporterLearnsBeforeItWrites(t *testing.T) {
	post := func(text string) models.WordPressPost {
		var p models.WordPressPost
		p.Excerpt.Rendered = excerptEndingIn(text, "続きを読む")

		return p
	}

	data := &models.ExportData{
		Posts: []models.WordPressPost{post("一"), post("二")},
		CustomTypes: []models.CustomTypeSet{
			{Slug: "services", Posts: []models.WordPressPost{post("三")}},
		},
	}

	exporter := NewExporter(&config.Config{})
	exporter.learnReadMore(data)

	assert.True(t, exporter.readMore.isReadMore("", "続きを読む"),
		"posts, pages and custom types all speak for the theme")
}
