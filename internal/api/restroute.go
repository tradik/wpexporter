package api

// The other spelling of the REST API (#66), and installs that have none (#68).
//
// WordPress serves its API two ways: the pretty `/wp-json/…`, and
// `/?rest_route=/…`, which works with no permalink structure at all and which
// WordPress documents as the fallback. Plenty of sites serve only the second —
// permalinks set to plain, or a security plugin hiding /wp-json/ because "the
// REST API is an attack surface" — and the exporter used to stop at the first
// 404 with a message about categories.
//
// This is the exception, not the rule, and it is priced that way: the pretty
// form is tried first and nothing is probed until it actually fails. A site
// that answers normally pays nothing at all, and the fallback costs one request
// on the site that needs it.
//
// A WordPress older than 4.7 has neither spelling of the content routes — wp/v2
// arrived in 4.7 — and answers `rest_no_route` to both. That is not a fallback
// to find but a fact to report, so the run says so once and carries on with
// what a site of that age still publishes: its sitemap and its feed (#68).

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"

	"github.com/go-resty/resty/v2"
)

// restRouteQuery is the fallback spelling's prefix.
const restRouteQuery = "/?rest_route="

// contentNamespace is where the content routes live. It is part of the API
// prefix in the pretty spelling and part of the route name in the fallback one,
// which is why both forms have to name it.
const contentNamespace = "/wp/v2"

// noRouteCode is what WordPress answers when the namespace does not exist —
// which on a pre-4.7 install is every content route there is.
const noRouteCode = "rest_no_route"

// routeProbe remembers what a client learned about a site's API, so the
// question is asked once per run rather than once per request.
type routeProbe struct {
	once     sync.Once
	fallback bool
	absent   bool
}

// endpointURL builds the address of one collection, in whichever spelling this
// site answers to.
//
// The query is passed separately because the two forms join it differently:
// `/wp-json/wp/v2/posts?page=1` against `/?rest_route=/wp/v2/posts&page=1`.
// Building the second by string concatenation is how a fallback quietly asks
// for `rest_route=/wp/v2/posts?page=1`, which WordPress reads as one route name
// and answers 404 to.
func (c *Client) endpointURL(path, query string) string {
	path = "/" + strings.TrimPrefix(path, "/")

	if !c.usesRestRouteFallback() {
		url := c.baseURL + path
		if query != "" {
			url += "?" + query
		}

		return url
	}

	url := c.siteRoot() + restRouteQuery + contentNamespace + path
	if query != "" {
		url += "&" + query
	}

	return url
}

// siteRoot is the address without the API prefix.
func (c *Client) siteRoot() string {
	return strings.TrimSuffix(strings.TrimSuffix(c.baseURL, contentNamespace), "/wp-json")
}

// apiRootURL is the API index — the document carrying the site's name, tagline
// and timezone. It has no namespace, so it is not an endpointURL: the fallback
// spelling of `/wp-json/` is `/?rest_route=/`, not `/?rest_route=/wp/v2/`.
func (c *Client) apiRootURL() string {
	if c.usesRestRouteFallback() {
		return c.siteRoot() + restRouteQuery + "/"
	}

	return strings.TrimSuffix(c.baseURL, contentNamespace)
}

// usesRestRouteFallback reports whether this site needs the query spelling. The
// probe runs at most once, and only after the pretty form has already failed.
func (c *Client) usesRestRouteFallback() bool {
	return c.probe.fallback
}

// UsesRestRouteFallback reports whether this run ended up addressing the site
// at ?rest_route=. It is worth saying in the report: the export is complete, but
// the address in it is not the one a reader would try by hand (#66).
func (c *Client) UsesRestRouteFallback() bool {
	return c.usesRestRouteFallback()
}

// RestAPIAbsent reports a site whose REST API has no content routes at all —
// a WordPress older than 4.7. There is nothing to fall back to, and the caller
// answers it with the sitemap and the feed rather than with another request.
func (c *Client) RestAPIAbsent() bool {
	return c.probe.absent
}

// discoverRoute is called the first time a request comes back 404, and decides
// which of three worlds this site is in: the pretty form works after all
// (nothing to do), the query form works (use it), or neither does (say so).
func (c *Client) discoverRoute() {
	c.probe.once.Do(func() {
		if c.answersContent(c.baseURL + "/types") {
			return
		}

		if c.answersContent(c.siteRoot() + restRouteQuery + contentNamespace + "/types") {
			c.probe.fallback = true

			return
		}

		c.probe.absent = true
	})
}

// answersContent reports whether an address serves a wp/v2 document rather than
// something that merely has its address.
//
// Three things can wear a 200 here, and only one of them is an API:
//
//   - a JSON document, which is the answer;
//   - a JSON refusal — `rest_no_route` — which is a no wearing a success, and
//     is what a WordPress older than 4.7 says about every content route;
//   - the site's own HTML, which is what a site with no REST API at all serves
//     to `/?rest_route=…`, because to that site it is a URL like any other.
//
// The third was read as an API until a reporter checked (#66). Everything then
// failed to parse, the run reported two incomplete collections and zero
// documents, and the note at the end said the export "is complete". A probe
// that trusts a status code will believe any 404 page that returns 200.
func (c *Client) answersContent(url string) bool {
	resp, err := c.httpClient.R().Get(url)
	if err != nil || resp.StatusCode() != http.StatusOK {
		return false
	}

	return isRESTDocument(resp.Body())
}

// isRESTDocument reports a body that is JSON and is not a refusal. A page of
// HTML is neither, whatever status it arrived with.
func isRESTDocument(body []byte) bool {
	// Validity is asked first and separately: a collection answers with a JSON
	// array, which will not unmarshal into the refusal's shape, and reading
	// that failure as "not an API" would reject every working endpoint.
	if !json.Valid(body) {
		return false
	}

	var refusal struct {
		Code string `json:"code"`
	}

	if err := json.Unmarshal(body, &refusal); err == nil && refusal.Code == noRouteCode {
		return false
	}

	return true
}

// RestAPINotice is what the run says about a site with no content routes, once.
//
// It states what happened and stops there. An earlier draft ended "the export
// used it and is complete", which on a site serving HTML to every API address
// printed directly beneath two incomplete collections and a 1.6 KB export
// (#66). A report may say what happened; it may not say what it concluded.
func RestAPINotice() string {
	return "This site serves no wp/v2 content routes: neither /wp-json/wp/v2/ nor " +
		"/?rest_route=/wp/v2/ answered with a REST document. That is a WordPress older than 4.7, " +
		"which predates the content API, or one with the API disabled or hidden. Nothing can be " +
		"fetched through it — what this export carries came from the site's sitemap and feed."
}

// fetchCollection asks for one page of a collection, and on a 404 asks once
// more in the other spelling.
//
// The probe runs at most once per client and only after a 404, so a site whose
// API answers normally — nearly all of them — never spends a request on the
// question. The retry is a single extra request on the site that needs it.
func (c *Client) fetchCollection(endpoint, query string) (*resty.Response, error) {
	return c.fetchProbing(func() string { return c.endpointURL(endpoint, query) })
}

// fetchProbing asks for an address and, if the answer is a 404, discovers which
// spelling this site answers to and asks once more when that changed the
// address. The builder is a function rather than a string because the second
// attempt has to be addressed in the spelling the probe just settled on.
func (c *Client) fetchProbing(address func() string) (*resty.Response, error) {
	resp, err := c.httpClient.R().Get(address())
	if err != nil || resp.StatusCode() != http.StatusNotFound {
		return resp, err
	}

	before := c.usesRestRouteFallback()
	c.discoverRoute()

	if c.usesRestRouteFallback() == before {
		// Either the pretty spelling was right all along and this 404 means
		// what it says, or the site has no content routes at all (#68).
		return resp, err
	}

	return c.httpClient.R().Get(address())
}
