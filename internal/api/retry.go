package api

// Retrying the source site (#37).
//
// A crawl of somebody else's WordPress is thousands of requests against a host
// nobody here controls. Shared hosting answers 500 or 503 under load, a proxy
// resets a connection, a firewall counts requests and returns 429 — and the
// same URL succeeds seconds later. Without retries a one-in-a-thousand blip
// ends a run that already downloaded hundreds of megabytes, which on a flaky
// site means it cannot be exported at all.

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
)

// The backoff bounds. resty jitters exponentially between them, so a burst of
// retries does not arrive at a struggling server in lockstep. The ceiling also
// caps a Retry-After the site sends: a header asking for an hour is a refusal
// to be reported, not a delay to sit through.
const (
	retryWaitTime    = 500 * time.Millisecond
	retryMaxWaitTime = 30 * time.Second
)

// maxContentPages bounds a collection walk. At 100 records a page that is a
// million documents — orders of magnitude past any WordPress install — so
// reaching it means the site is repeating itself rather than paginating, and
// the walk stops with what it has instead of running until memory does.
const maxContentPages = 10000

// transientStatuses are the answers worth repeating: the server is busy,
// broken, restarting or rate-limiting, and the same request may well succeed.
//
// Every other status is an answer rather than weather — the route is gone, the
// credentials are wrong, the page number is past the end — and repeating the
// request cannot change it. 501 and 505 are 5xx that mean exactly that, which
// is why this is a list and not `>= 500`.
var transientStatuses = map[int]struct{}{
	http.StatusRequestTimeout:      {},
	http.StatusTooManyRequests:     {},
	http.StatusInternalServerError: {},
	http.StatusBadGateway:          {},
	http.StatusServiceUnavailable:  {},
	http.StatusGatewayTimeout:      {},
	http.StatusInsufficientStorage: {},
}

// isTransientFailure decides what deserves another attempt.
//
// The transport case is stated explicitly because resty's condition list
// *replaces* its built-in "retry on transport error" rule rather than adding to
// it: registering this function without the nil-error branch would quietly stop
// connection resets — the failure this exists for — from ever being retried.
func isTransientFailure(resp *resty.Response, err error) bool {
	if err != nil {
		return true
	}

	if resp == nil {
		return false
	}

	_, transient := transientStatuses[resp.StatusCode()]

	return transient
}

// retryAfterDelay honours the server's own Retry-After when it sends one, in
// either form the header allows: a number of seconds, or an HTTP date.
//
// Returning (0, nil) means "use the backoff", which is what an absent, expired
// or unreadable header gets — the request is still retried, just on our own
// schedule. A non-nil error would abandon the retry entirely, so this never
// returns one: a malformed header is the site's mistake, not a reason to stop.
func retryAfterDelay(_ *resty.Client, resp *resty.Response) (time.Duration, error) {
	if resp == nil {
		return 0, nil
	}

	header := strings.TrimSpace(resp.Header().Get("Retry-After"))
	if header == "" {
		return 0, nil
	}

	if seconds, err := strconv.Atoi(header); err == nil {
		if seconds <= 0 {
			return 0, nil
		}

		return time.Duration(seconds) * time.Second, nil
	}

	if when, err := http.ParseTime(header); err == nil {
		if delay := time.Until(when); delay > 0 {
			return delay, nil
		}
	}

	return 0, nil
}

// PartialError reports a collection that could not be read to the end.
//
// It carries what was fetched before the gap, because the alternative — one
// unreadable page discarding every record already in hand — is how a flaky
// site becomes an unexportable one. An export missing a hundred posts and
// saying so is worth more than no export (#37).
type PartialError struct {
	// Endpoint is the collection that broke off ("posts", "pages", "media").
	Endpoint string
	// Page is the page of results that could not be read.
	Page int
	// Fetched counts the records retrieved before the failure.
	Fetched int
	// Err is what the last attempt reported, after the retries.
	Err error
}

func (e *PartialError) Error() string {
	return fmt.Sprintf("%s: stopped at page %d after %d records: %v",
		e.Endpoint, e.Page, e.Fetched, e.Err)
}

func (e *PartialError) Unwrap() error { return e.Err }

// Gap describes err as an incomplete read, and reports whether it was one.
//
// It exists so every caller draws the same line in the same place: a hole in a
// collection is reported and survived, anything else ends the export. The CLI
// and the MCP server used to be free to disagree about that, which is how one
// entry point can quietly lose what the other refuses to.
func Gap(err error) (string, bool) {
	var partial *PartialError
	if errors.As(err, &partial) {
		return partial.Error(), true
	}

	return "", false
}
