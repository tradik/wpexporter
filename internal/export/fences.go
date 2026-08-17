package export

// Code fences that stay where they were put (#69).
//
// `<pre>` became "```" and `</pre>` became "```", wherever they happened to
// sit. A plugin that ships a `<pre>` inside a `<div>` — a reviews widget, a
// code-sample block, anything with a `<template>` in it — therefore had a fence
// opened mid-element and closed mid-element, and everything between them, plus
// everything the closing marker pushed out of alignment after them, rendered as
// source code: a grey box a screen and a half tall on a published page.
//
// Three things keep a fence where it belongs:
//
//   - a marker sits on a line of its own, because a fence that does not start a
//     line is not a fence in CommonMark and a fence that shares a line with
//     other text swallows it;
//   - a fence is longer than the longest run of backticks inside it, which is
//     the rule CommonMark states for exactly this case — content carrying ```
//     would otherwise close its own block early;
//   - a document never ends inside a fence, because an unclosed one takes the
//     rest of the page with it, and on an SSG's index it takes the next page's
//     content too.

import (
	"regexp"
	"strings"
)

// preBlockRe matches one <pre> element and its contents. Non-greedy, so nested
// markup after the first </pre> is not swallowed.
var preBlockRe = regexp.MustCompile(`(?is)<pre\b[^>]*>(.*?)</pre\s*>`)

// backtickRunRe finds the longest run of backticks in a block's content.
var backtickRunRe = regexp.MustCompile("`+")

// spaceBeforeFenceRe finds what the <pre> left behind on the line above it —
// the space in "and <pre>". Trailing whitespace is not inert in Markdown: two
// of them are a hard line break, and here they sit where the reader can neither
// see nor delete them.
var spaceBeforeFenceRe = regexp.MustCompile("[ \t]+\n\n(`{3,})")

// minFence is CommonMark's shortest fence.
const minFence = 3

// convertPreBlocks turns every <pre> into a fenced block whose markers own
// their lines and whose fence is long enough to hold the content.
func convertPreBlocks(html string) string {
	fenced := preBlockRe.ReplaceAllStringFunc(html, func(block string) string {
		content := preBlockRe.FindStringSubmatch(block)[1]

		// A block with nothing in it is nothing. This is not a hypothetical:
		// a <pre> wrapping a plugin's <template> is empty once the template is
		// dropped, and an empty fence puts a grey box on the page for content
		// that was never there (#69).
		if strings.TrimSpace(content) == "" {
			return "\n\n"
		}

		fence := strings.Repeat("`", fenceLength(content))

		// The blank lines matter as much as the fence: a marker glued to the
		// end of a <div> is not at the start of a line, and a renderer either
		// ignores it — leaving backticks on the page — or takes the rest of the
		// document with it.
		return "\n\n" + fence + "\n" + strings.Trim(content, "\n") + "\n" + fence + "\n\n"
	})

	return spaceBeforeFenceRe.ReplaceAllString(fenced, "\n\n$1")
}

// fenceLength is one longer than the longest backtick run inside, so the block
// cannot close itself early.
func fenceLength(content string) int {
	longest := 0
	for _, run := range backtickRunRe.FindAllString(content, -1) {
		if len(run) > longest {
			longest = len(run)
		}
	}

	if longest < minFence {
		return minFence
	}

	return longest + 1
}

// closeDanglingFence closes a document that ends inside a fence.
//
// Malformed markup — a <pre> with no closing tag, which page builders emit —
// would otherwise turn every line after it, and on a generator's index the next
// document too, into code.
func closeDanglingFence(md string) string {
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
		return md
	}

	return strings.TrimRight(md, "\n") + "\n" + open + "\n"
}
