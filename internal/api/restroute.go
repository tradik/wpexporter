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
// a refusal. A 200 carrying `rest_no_route` is a refusal wearing a success.
func (c *Client) answersContent(url string) bool {
	resp, err := c.httpClient.R().Get(url)
	if err != nil || resp.StatusCode() != http.StatusOK {
		return false
	}

	var refusal struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(resp.Body(), &refusal); err == nil && refusal.Code == noRouteCode {
		return false
	}

	return true
}

// RestAPINotice is what the run says about a site with no content routes, once.
func RestAPINotice() string {
	return "This WordPress serves no wp/v2 content routes — the REST content API arrived in " +
		"WordPress 4.7, and this install predates it or has it disabled. Nothing can be fetched " +
		"through the API; --from-sitemap reads what the site still publishes in its feed."
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
