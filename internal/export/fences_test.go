package export

// A fence that stays where it was put (#69).

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFenceDoesNotSwallowThePage: the case from the issue — a plugin's <pre>
// inside a <div>, with a <template> in it. Everything after the widget used to
// render as source code.
func TestFenceDoesNotSwallowThePage(t *testing.T) {
	out := htmlToMarkdown(`<div class="elementor-shortcode"><pre><template id="w">x</template></pre>` +
		`<div data-src="https://cdn/loader.js"></div></div><p>After the widget.</p>`)

	assert.Contains(t, out, "After the widget.")

	lines := strings.Split(out, "\n")
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			assert.Equal(t, strings.TrimSpace(line), line,
				"line %d: a fence marker owns its line", i)
		}
	}

	assert.Equal(t, 0, countOpenFences(out), "the document does not end inside a fence")
}

// TestFenceIsLongerThanItsContents: CommonMark's own rule for this case. A
// block carrying ``` would otherwise close itself early and spill the rest.
func TestFenceIsLongerThanItsContents(t *testing.T) {
	out := htmlToMarkdown("<pre>a ``` b</pre>")

	assert.Contains(t, out, "````\na ``` b\n````")
	assert.Equal(t, 0, countOpenFences(out))
}

// TestUnclosedPreIsClosed: page builders emit half a <pre>. Left open, it takes
// the rest of the page — and on a generator's index, the next document too.
func TestUnclosedPreIsClosed(t *testing.T) {
	out := htmlToMarkdown(`<p>Before</p><pre>code without an end`)

	assert.Contains(t, out, "Before")
	assert.Contains(t, out, "code without an end")
	assert.Equal(t, 0, countOpenFences(out), "an unclosed block is closed rather than left open")
}

// TestOrdinaryCodeBlockIsUnchanged: the common case must convert as it always
// did, or every existing export is a diff.
func TestOrdinaryCodeBlockIsUnchanged(t *testing.T) {
	out := htmlToMarkdown("<pre>go build ./...</pre>")

	assert.Equal(t, "```\ngo build ./...\n```", strings.TrimSpace(out))
}

// TestFenceLength covers the arithmetic on its own.
func TestFenceLength(t *testing.T) {
	assert.Equal(t, 3, fenceLength("plain"))
	assert.Equal(t, 4, fenceLength("a ``` b"))
	assert.Equal(t, 5, fenceLength("a ```` b"))
	assert.Equal(t, 3, fenceLength("one ` and two ``"))
}

// TestCloseDanglingFence: the guard on its own, including the case where a
// longer marker closes a shorter one.
func TestCloseDanglingFence(t *testing.T) {
	assert.Equal(t, "text", closeDanglingFence("text"), "a document with no fence is untouched")

	closed := closeDanglingFence("```\ncode\n```")
	assert.Equal(t, "```\ncode\n```", closed, "a balanced document is untouched")

	dangling := closeDanglingFence("```\ncode")
	assert.True(t, strings.HasSuffix(dangling, "```\n"))

	require.Equal(t, 0, countOpenFences(closeDanglingFence("````\ncode\n```")),
		"a shorter marker cannot close a longer fence, so the document is closed")
}

// countOpenFences reports how many fences a document leaves open.
func countOpenFences(md string) int {
	open := ""

	for _, line := range strings.Split(md, "\n") {
		marker := strings.TrimRight(line, " \t")
		if !strings.HasPrefix(marker, "```") || strings.TrimLeft(marker, "`") != "" {
			continue
		}

		switch {
		case open == "":
			open = marker
		case len(marker) >= len(open):
			open = ""
		}
	}

	if open == "" {
		return 0
	}

	return 1
}

// TestTemplateIsNotContent: the verification that reopened #69. The fences were
// well formed after the first fix and still there, so the widget's markup
// arrived on the page as a grey box of source. A <template> is what a plugin
// clones at run time — it renders nothing where it sits, and it is no more
// content than a <script> is.
func TestTemplateIsNotContent(t *testing.T) {
	out := htmlToMarkdown(`<p>Reviews</p><div class="wp-review-widget">` +
		`<pre><template id="tpl"><div class="review">{{author}}</div></template></pre>` +
		`</div><p>After the widget.</p>`)

	assert.NotContains(t, out, "```", "nothing is left to fence")
	assert.NotContains(t, out, "{{author}}")
	assert.Contains(t, out, "Reviews")
	assert.Contains(t, out, "After the widget.")
}

// TestPreWithRealCodeStillFences: the other half of the same rule. A <pre> that
// holds code holds content, and dropping it would lose what a documentation
// page is for.
func TestPreWithRealCodeStillFences(t *testing.T) {
	out := htmlToMarkdown("<p>Run:</p><pre>go build ./...</pre>")

	assert.Contains(t, out, "```")
	assert.Contains(t, out, "go build ./...")
}
