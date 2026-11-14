package api

// Exporting less than everything, in the shape asked for (#60, #62).
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
//
// The budget is per kind as well as overall, because a preview has a shape:
// five posts say what a blog is, and five media items say almost nothing about
// a gallery site (#62).

// Collection names the walks a budget can be given to. They are the strings an
// operator types, so they are also the strings the report prints.
const (
	CollectionPosts    = "posts"
	CollectionPages    = "pages"
	CollectionMedia    = "media"
	CollectionProducts = "products"
)

// Limits is how much an export may still fetch. One value serves every walk of
// a run — the REST client's, and in local mode the database's as well — so a
// total budget is spent across all of them, in the order they run. The walks
// run one after another; Limits is not safe for concurrent use. A nil *Limits
// is no limit at all.
type Limits struct {
	// perType bounds one named collection. A custom type is named by its slug.
	perType map[string]int
	// defaultPerType bounds every kind perType does not name; 0 is unbounded.
	defaultPerType int
	// remaining is the shared budget across every collection, spent in fetch
	// order. Meaningful only when bounded.
	remaining int
	// bounded distinguishes "no total budget" from "a total budget of nothing".
	bounded bool
}

// NewLimits reads the caps into the form the walks use: --limit, the default
// per kind, and the kinds named on their own.
func NewLimits(total, defaultPerType int, perType map[string]int) *Limits {
	capped := make(map[string]int, len(perType))
	for name, value := range perType {
		if value > 0 {
			capped[name] = value
		}
	}

	return &Limits{
		perType:        capped,
		defaultPerType: maxZero(defaultPerType),
		remaining:      maxZero(total),
		bounded:        total > 0,
	}
}

// budget is how many records the named collection may take: the smaller of what
// is left overall and what this kind may have. Zero means unbounded.
func (l *Limits) budget(collection string) int {
	perType := l.defaultPerType
	if named, ok := l.perType[collection]; ok {
		perType = named
	}

	switch {
	case l.bounded && perType > 0:
		return min(l.remaining, perType)
	case l.bounded:
		return l.remaining
	default:
		return perType
	}
}

// Take returns how many records the named collection may read (zero is
// unbounded), and whether its walk should start at all. It is the one question
// every walk asks first, so every collection is capped the same way — media
// and products included, which they were not when the budget lived inside a
// single walk (#62).
func (l *Limits) Take(collection string) (budget int, proceed bool) {
	if l == nil {
		return 0, true
	}
	if l.exhausted() {
		return 0, false
	}

	return l.budget(collection), true
}

// Spend records what a walk took, so the next one sees the smaller budget. A
// per-kind limit spends nothing: each kind has its own.
func (l *Limits) Spend(count int) {
	if l == nil || !l.bounded {
		return
	}

	l.remaining -= count
	if l.remaining < 0 {
		l.remaining = 0
	}
}

// exhausted reports a total budget with nothing left, which is how a walk knows
// to stop asking rather than to ask for zero records.
func (l *Limits) exhausted() bool {
	return l.bounded && l.remaining == 0
}

// Active reports whether anything is capped at all.
func (l *Limits) Active() bool {
	return l != nil && (l.bounded || l.defaultPerType > 0 || len(l.perType) > 0)
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
	return c.limits.Active()
}

// ShareLimits makes the client spend from l instead of a budget of its own. In
// local mode the database's walks and this client's products draw on one
// --limit, which two separate budgets would grant twice.
func (c *Client) ShareLimits(l *Limits) {
	c.limits = l
}

// takeBudget returns how many records the named collection may fetch, and
// whether the walk should start at all.
func (c *Client) takeBudget(collection string) (budget int, proceed bool) {
	return c.limits.Take(collection)
}

// spendBudget records what a walk took.
func (c *Client) spendBudget(count int) {
	c.limits.Spend(count)
}
