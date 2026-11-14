package api

// Reading a collection to the end, and knowing when it was not (#43).
//
// A 400 used to end a walk silently, because that is what WordPress answers
// past the last page. It is also what it answers when a request asks for more
// records per page than the site allows — a cap a plugin or a security layer
// can set below the REST default — and the two were indistinguishable. A type
// whose collection refused `per_page=100` therefore exported as zero records,
// with no error and no warning: `--custom-types mec-events` on a site with 56
// events brought none of them, and the report said nothing at all.
//
// WordPress names its refusals, so they are now told apart: past-the-end ends
// the walk, a rejected page size is retried smaller, and anything else is a gap
// the export reports. The site also states the size of every collection in a
// header, which is the second half of the answer: a walk that ends early with
// fewer records than the site claims says so rather than reporting success.

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/tradik/wpexporter/pkg/models"
)

const (
	// codePageBeyondEnd is WordPress's own name for "you asked past the last
	// page", which is how a walk normally ends.
	codePageBeyondEnd = "rest_post_invalid_page_number"
	// codeInvalidParam is what it answers when a parameter is out of range —
	// most often per_page, against a site that caps it below the REST default.
	codeInvalidParam = "rest_invalid_param"
	// totalHeader carries the size of the collection the site is serving.
	totalHeader = "X-WP-Total"
)

// pageSizes are tried in order. 100 is the REST maximum and the fastest walk;
// the rest exist because a site that refuses it usually accepts something
// smaller, and 56 records fetched ten at a time is still 56 records.
var pageSizes = []int{100, 50, 25, 10, 5, 1}

// restRefusal is WordPress's error body, reduced to the name of the refusal.
type restRefusal struct {
	Code string `json:"code"`
}

// refusalCode reads the site's own name for a refusal, or "" when the body is
// not one of its error documents.
func refusalCode(body []byte) string {
	var refusal restRefusal
	if err := json.Unmarshal(body, &refusal); err != nil {
		return ""
	}

	return refusal.Code
}

// collectionTotal reads the number of records the site says the collection
// holds. Zero means it did not say — an older site, a plugin that strips the
// header, or a route that does not send one — and a number nobody stated is not
// a number to check against.
func collectionTotal(header string) int {
	total, err := strconv.Atoi(strings.TrimSpace(header))
	if err != nil || total < 0 {
		return 0
	}

	return total
}

// nextPageSize returns the size to try after one the site refused, and whether
// there is one left to try.
func nextPageSize(current int) (int, bool) {
	for i, size := range pageSizes {
		if size == current && i+1 < len(pageSizes) {
			return pageSizes[i+1], true
		}
	}

	return current, false
}

// pageResult is what one request for a page of a collection yielded: records,
// the end of the walk, a page size to try instead, or a gap to report.
type pageResult struct {
	content []models.WordPressPost
	// total is the site's own count for the collection, 0 when it did not say.
	total int
	// pages is the site's own page count at this size, 0 when it did not say.
	pages int
	// done reports that the collection ended here.
	done bool
	// retryWith is a smaller page size the site may accept, or 0.
	retryWith int
	// err is the gap, carrying what had been fetched before it.
	err error
}

// refusal is what a 400 means for a walk in progress.
type refusal struct {
	// retryWith is the page size to try instead, or 0 when there is none.
	retryWith int
	// done reports the ordinary end of a collection.
	done bool
	// code is the site's own name for the refusal, for the report.
	code string
}

// classifyRefusal reads a 400 and decides what the walk should do.
//
// Past the last page is how every walk ends. A rejected page size is worth one
// more attempt at a smaller one — but only before any record has been read,
// because page numbers are relative to the size, and shrinking it mid-walk
// would re-read some records and skip others. Anything else is a refusal the
// export should report rather than mistake for an empty collection (#43).
func classifyRefusal(body []byte, perPage, fetched int) refusal {
	code := refusalCode(body)

	if code == codeInvalidParam && fetched == 0 {
		if smaller, ok := nextPageSize(perPage); ok {
			return refusal{retryWith: smaller, code: code}
		}
	}

	if code == "" || code == codePageBeyondEnd {
		return refusal{done: true, code: code}
	}

	return refusal{code: code}
}

// Ending a walk the site would not end (#37 for posts and pages; every other
// walk followed a site that ignores ?page= until memory ran out).
//
// A server that ignores ?page= answers every page with the first one. Asking
// for page N+1 then returns the records already kept, for ever: nothing in the
// answer is empty and nothing is refused. Three things end a walk now, the
// same way for every collection: the page count the site states in
// X-WP-TotalPages, a page whose records are those of the page before — the
// repeat, which needs no header to be seen — and the page ceiling.

// totalPagesHeader carries how many pages the collection spans at this size.
const totalPagesHeader = "X-WP-TotalPages"

// errPageRepeated is the repeat, reported as a gap: what lies past the first
// page could not be read.
var errPageRepeated = errors.New("the site answered with the records of the page before — it ignores ?page=, " +
	"so nothing past that page could be read")

// pager follows one walk and says when it is over.
type pager struct {
	endpoint string
	previous []int
}

// check refuses a page whose records are those of the page before. Its records
// are not kept.
func (p *pager) check(page int, ids []int, fetched int) error {
	if p.previous != nil && slices.Equal(ids, p.previous) {
		return &PartialError{Endpoint: p.endpoint, Page: page, Fetched: fetched, Err: errPageRepeated}
	}
	p.previous = ids
	return nil
}

// done reports whether the page just kept was the last: the site's own page
// count is reached (totalPages is 0 when it did not say), or the ceiling, which
// is a gap.
func (p *pager) done(page, totalPages, fetched int) (bool, error) {
	switch {
	case totalPages > 0 && page >= totalPages:
		return true, nil
	case page >= maxContentPages:
		return true, &PartialError{Endpoint: p.endpoint, Page: page + 1, Fetched: fetched,
			Err: fmt.Errorf("collection did not end within %d pages", maxContentPages)}
	default:
		return false, nil
	}
}

// take keeps a page's records unless they repeat the page before, and reports
// whether the walk is over. It is the one step every walk takes per page.
func take[T any](p *pager, all *[]T, items []T, id func(T) int, page, totalPages int) (bool, error) {
	ids := make([]int, len(items))
	for i, item := range items {
		ids[i] = id(item)
	}
	if err := p.check(page, ids, len(*all)); err != nil {
		return true, err
	}
	*all = append(*all, items...)
	return p.done(page, totalPages, len(*all))
}

// pageCount reads X-WP-TotalPages; 0 when the site did not say.
func pageCount(h http.Header) int {
	return collectionTotal(h.Get(totalPagesHeader))
}

// The record IDs a repeat is recognized by.
func postID(p models.WordPressPost) int         { return p.ID }
func mediaID(m models.WordPressMedia) int       { return m.ID }
func categoryID(c models.WordPressCategory) int { return c.ID }
func tagID(t models.WordPressTag) int           { return t.ID }
func userID(u models.WordPressUser) int         { return u.ID }
func productID(p models.WooCommerceProduct) int { return p.ID }
func commentID(c models.WordPressComment) int   { return c.ID }
