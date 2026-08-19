package api

// A 200 that is not the API (#73).
//
// `failed to parse response: invalid character '<' looking for beginning of
// value` is what this said when a collection answered 200 with a page of HTML,
// and it is a true sentence that helps nobody. Three different things wear that
// shape, and an operator can act on each of them differently:
//
//   - a wall — Cloudflare or another bot protection — serving its block or
//     challenge page with a 200, which is what a shop behind managed rules does
//     to a client calling itself WordPress-Export-JSON/1.0 (#58);
//   - a WordPress with no REST API, which serves the site's own home page to
//     any /wp-json/ address, because to it that is a URL like any other (#66);
//   - a plugin or a theme printing a notice before the JSON, which leaves a
//     document that starts with a warning and parses as neither.
//
// Naming which one it is turns an unreadable line into an instruction. It also
// stops the products report concluding "published none" about a route that
// answered perfectly well and was never read (#73).

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/go-resty/resty/v2"
)

// htmlOpening are the marks of a document that is a page rather than a payload.
var htmlOpening = [][]byte{
	[]byte("<!doctype"), []byte("<html"), []byte("<head"), []byte("<body"), []byte("<br"),
}

// notJSONPeek is how much of a body is read to tell a page from a payload.
// A wall's block page announces itself in its first tag; anything further in is
// the page's own content.
const notJSONPeek = 2048

// ErrNotJSON marks a body that arrived with a success status and is not the
// API's. Callers ask about it to tell "this route has nothing" from "this route
// was never really read".
var ErrNotJSON = errors.New("the address answered with a page rather than a REST document")

// unreadableBody explains a 200 that would not parse, naming the wall when the
// body carries one's marks and saying plainly that it is a page otherwise.
//
// The original parse error travels underneath: it is what a developer needs and
// what an operator cannot use, so it is available and not in the way.
func unreadableBody(resp *resty.Response, parseErr error) error {
	if !looksLikeHTML(resp.Body()) {
		return fmt.Errorf("failed to parse response: %w", parseErr)
	}

	if advice := InspectRefusal(resp).Advice(); advice != "" {
		return fmt.Errorf("%w — %s (status %d)", ErrNotJSON, advice, resp.StatusCode())
	}

	return fmt.Errorf("%w: status %d carried HTML, so this address is served by the site "+
		"rather than by its REST API — a WordPress with no REST routes answers its own pages "+
		"here, and so does a wall in front of one", ErrNotJSON, resp.StatusCode())
}

// looksLikeHTML reports a body that opens as a page.
func looksLikeHTML(body []byte) bool {
	peek := body
	if len(peek) > notJSONPeek {
		peek = peek[:notJSONPeek]
	}

	peek = bytes.ToLower(bytes.TrimSpace(peek))

	for _, opening := range htmlOpening {
		if bytes.HasPrefix(peek, opening) {
			return true
		}
	}

	return false
}
