package api

// Exporting less than everything (#60).
//
// The smallest export of a site used to be the whole site. Every flag that
// bounded a run bounded something else — an ID range, how far brute force
// walks, one file's size, a whole kind of content — so a preview of the first
// five pages downloaded five hundred, which is slow for whoever asked, unkind
// to the source host and expensive for whoever pays for the bandwidth.
//
// A limit is applied while walking rather than after it: the point is not to
// hand back fewer records, it is not to fetch them. The walk stops as soon as
// its budget is spent, which on a 500-page site is one request instead of five.
//
// Newest first, because the first five posts of a blog say more about it than
// five arbitrary ones — and because a truncated export that cannot say what it
// truncated is the silent-cap failure this must not become.

// limits is how much a client may still fetch.
type limits struct {
	// perType bounds each collection on its own: posts, pages, and each custom
	// type get this many. Zero means unbounded.
	perType int
	// remaining is the shared budget across every collection, spent in fetch
	// order. Zero after initialisation means unbounded; it is set negative
	// never, since a spent budget is exactly zero.
	remaining int
	// bounded distinguishes "no total budget" from "a total budget of nothing".
	bounded bool
}

// newLimits reads the two caps into the form the walk uses.
func newLimits(total, perType int) limits {
	return limits{
		perType:   maxZero(perType),
		remaining: maxZero(total),
		bounded:   total > 0,
	}
}

// budget is how many records the next collection may take: the smaller of what
// is left overall and what one collection may have. Zero means unbounded.
func (l limits) budget() int {
	switch {
	case l.bounded && l.perType > 0:
		return min(l.remaining, l.perType)
	case l.bounded:
		return l.remaining
	default:
		return l.perType
	}
}

// spend records what a collection took, so the next one sees the smaller
// budget. A per-type-only limit spends nothing: each collection has its own.
func (l *limits) spend(count int) {
	if !l.bounded {
		return
	}

	l.remaining -= count
	if l.remaining < 0 {
		l.remaining = 0
	}
}

// exhausted reports a total budget with nothing left, which is how the walk
// knows to stop asking rather than to ask for zero records.
func (l limits) exhausted() bool {
	return l.bounded && l.remaining == 0
}

// active reports whether anything is capped at all.
func (l limits) active() bool {
	return l.bounded || l.perType > 0
}

// maxZero keeps a negative flag value from reading as a budget.
func maxZero(value int) int {
	if value < 0 {
		return 0
	}

	return value
}

// recordStated remembers what the site said a collection holds, so a truncated
// export can say what it truncated: "Posts: 5 (limited from 75)" is the line
// that keeps a preview from being mistaken for a complete export.
func (c *Client) recordStated(endpoint string, total int) {
	if total <= 0 {
		return
	}

	c.statedMu.Lock()
	defer c.statedMu.Unlock()

	if c.stated == nil {
		c.stated = map[string]int{}
	}

	c.stated[endpoint] = total
}

// StatedTotal is the size the site claimed for a collection, or 0 when it did
// not say. The header is not always there — an older site, a plugin that strips
// it — and a number nobody stated is not a number to print.
func (c *Client) StatedTotal(endpoint string) int {
	c.statedMu.Lock()
	defer c.statedMu.Unlock()

	return c.stated[endpoint]
}

// Limited reports whether this client was asked to cap the export.
func (c *Client) Limited() bool {
	return c.limits.active()
}
