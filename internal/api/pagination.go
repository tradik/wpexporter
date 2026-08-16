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
