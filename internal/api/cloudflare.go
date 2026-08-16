package api

// Naming the wall (#58).
//
// A site behind Cloudflare's bot protection answers 403, or serves a challenge
// page, and the export reported `API returned status 403` — accurate, and
// useless. Nothing said the refusal came from Cloudflare rather than from
// WordPress, so the operator was left to work out that a 403 on a route their
// browser opens fine is a firewall rule and not a broken REST API.
//
// Cloudflare identifies itself in every response it serves: a `cf-ray` header,
// `server: cloudflare`, and an error code in the body of a block page. When
// those are there the export says so, and names the remedies that apply to a
// wall rather than the ones that apply to a bug.

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/go-resty/resty/v2"
)

// cloudflareErrorRe matches the code Cloudflare prints on its block pages —
// 1020 for a firewall rule, 1010 for a refused client, 1015 for rate limiting.
var cloudflareErrorRe = regexp.MustCompile(`(?i)error code[: ]+(\d{4})`)

// challengeMarkers appear in the interstitials a browser is expected to solve.
// A retry cannot solve one, and knowing that is worth more than three backoff
// waits spent finding out.
//
// "Attention Required!" is deliberately absent: it is the title of the *block*
// page, not of a challenge, and reading a block as a challenge would report the
// wrong thing about the wall — a block is permanent for this address, while a
// challenge is a door a browser can open.
var challengeMarkers = []string{
	"cf-browser-verification", "cf_chl_", "checking your browser", "just a moment",
}

// Refusal describes a response that a wall produced rather than the site.
type Refusal struct {
	// ByCloudflare reports whether Cloudflare served the response.
	ByCloudflare bool
	// Code is Cloudflare's own error number, when its block page carried one.
	Code string
	// Challenge reports an interstitial a browser is expected to solve, which
	// no number of retries will pass.
	Challenge bool
}

// Advice is what to say to the operator: what refused, and what to change.
//
// The remedies are ordered by how often they work, and none of them is "try
// again" — a wall is not weather.
func (r Refusal) Advice() string {
	if !r.ByCloudflare {
		return ""
	}

	subject := "Cloudflare refused the request"
	if r.Code != "" {
		subject += " (error " + r.Code + ")"
	}
	if r.Challenge {
		subject = "Cloudflare served a browser challenge"
	}

	return subject + " — this is the site's bot protection, not its REST API. " +
		"Try --user-agent with a browser's string, --rate-limit to slow the crawl, " +
		"or ask the site to allow this address; --auth-user/--auth-pass helps only " +
		"where the block is WordPress's own."
}

// InspectRefusal reads a response for the marks of a wall.
//
// Only the head of the body is examined: a block page announces itself in its
// first kilobyte, and a challenge document can be large.
func InspectRefusal(resp *resty.Response) Refusal {
	if resp == nil {
		return Refusal{}
	}

	headers := resp.Header()
	byCloudflare := headers.Get("cf-ray") != "" ||
		strings.EqualFold(headers.Get("server"), "cloudflare")

	body := resp.Body()
	if len(body) > 4096 {
		body = body[:4096]
	}
	lowered := strings.ToLower(string(body))

	refusal := Refusal{ByCloudflare: byCloudflare}

	if match := cloudflareErrorRe.FindStringSubmatch(lowered); match != nil {
		refusal.Code = match[1]
		refusal.ByCloudflare = true
	}

	// A page carrying an error code is a block; only a page without one can be
	// the interstitial.
	if refusal.Code == "" {
		for _, marker := range challengeMarkers {
			if strings.Contains(lowered, marker) {
				refusal.Challenge = true
				refusal.ByCloudflare = true

				break
			}
		}
	}

	return refusal
}

// statusReason describes an unexpected status, naming the wall when a wall
// produced it. The status alone sent operators looking for a broken REST API.
func statusReason(resp *resty.Response) error {
	if advice := InspectRefusal(resp).Advice(); advice != "" {
		return fmt.Errorf("API returned status %d — %s", resp.StatusCode(), advice)
	}

	return fmt.Errorf("API returned status %d", resp.StatusCode())
}
