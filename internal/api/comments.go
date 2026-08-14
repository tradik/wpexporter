package api

// Reader comments (#35). WordPress serves them from /wp/v2/comments without
// authentication, but only the approved ones — which is exactly what a
// migration wants: pending and spam rows are moderation state, not content.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"

	"github.com/tradik/wpexporter/internal/cache"
	"github.com/tradik/wpexporter/pkg/models"
)

// ErrCommentsNotAccessible reports that the site does not serve its comments
// over REST — the route is disabled or gated. Like menus, this is a normal
// outcome for some installations rather than a failure: the caller warns and
// carries on with the rest of the export.
var ErrCommentsNotAccessible = fmt.Errorf("comments are not readable over the REST API")

// ErrCommentsDisabled reports that the site has commenting switched off
// entirely — WordPress answers 403 rest_comment_disabled. It is told apart from
// ErrCommentsNotAccessible because the remedies differ: credentials open a
// gated route, and nothing opens a route to comments that do not exist.
var ErrCommentsDisabled = fmt.Errorf("the site has comments disabled")

// restCommentDisabledCode is WordPress's own name for "commenting is off".
const restCommentDisabledCode = "rest_comment_disabled"

// commentsPerPage is the maximum WordPress allows for one page of comments.
const commentsPerPage = 100

// restComment is the /wp/v2/comments payload. `status` is an edit-context
// field, so a public read leaves it empty — the collection lists approved
// comments only, which is what defaultCommentStatus records.
type restComment struct {
	ID         int                  `json:"id"`
	Post       int                  `json:"post"`
	Parent     int                  `json:"parent"`
	AuthorName string               `json:"author_name"`
	AuthorURL  string               `json:"author_url"`
	Date       models.WordPressTime `json:"date"`
	DateGMT    models.WordPressTime `json:"date_gmt"`
	Content    rendered             `json:"content"`
	Link       string               `json:"link"`
	Status     string               `json:"status"`
	Type       string               `json:"type"`
	AvatarURLs map[string]string    `json:"author_avatar_urls"`
}

// defaultCommentStatus is what an unauthenticated read implies: WordPress only
// lists approved comments to the public.
const defaultCommentStatus = "approved"

// GetComments retrieves every comment the site publishes, oldest first so a
// reply never precedes the comment it answers when the export is replayed.
//
// It returns ErrCommentsNotAccessible when the route is switched off or gated,
// which some installations do; the export then continues without comments.
func (c *Client) GetComments() ([]models.WordPressComment, error) {
	cacheKey := cache.GenerateAPIKey("comments", 0)

	var cachedComments []models.WordPressComment
	if c.getFromCache(cacheKey, &cachedComments) {
		return cachedComments, nil
	}

	var all []models.WordPressComment

	for page := 1; ; page++ {
		if page > 1 {
			c.applyRateLimit()
		}

		batch, err := c.fetchCommentsPage(page)
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}

		all = append(all, batch...)

		if len(batch) < commentsPerPage {
			break
		}
	}

	// Threads only reconstruct in creation order: a reply carries its parent's
	// id, and a consumer inserting rows one by one needs the parent first.
	sort.SliceStable(all, func(i, j int) bool { return all[i].ID < all[j].ID })

	c.saveToCache(cacheKey, all)

	return all, nil
}

// fetchCommentsPage reads one page of the comment collection.
func (c *Client) fetchCommentsPage(page int) ([]models.WordPressComment, error) {
	url := fmt.Sprintf("%s/comments?page=%d&per_page=%d&order=asc&orderby=date",
		c.baseURL, page, commentsPerPage)

	resp, err := c.httpClient.R().Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get comments page %d: %w", page, err)
	}

	switch resp.StatusCode() {
	case http.StatusOK:
	case http.StatusBadRequest:
		// Past the last page — WordPress answers rest_post_invalid_page_number.
		return nil, nil
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound:
		return nil, refusalReason(resp.Body())
	default:
		return nil, fmt.Errorf("API returned status %d for comments page %d", resp.StatusCode(), page)
	}

	var restComments []restComment
	if err := json.Unmarshal(resp.Body(), &restComments); err != nil {
		return nil, fmt.Errorf("failed to parse comments response: %w", err)
	}

	comments := make([]models.WordPressComment, 0, len(restComments))
	for _, rc := range restComments {
		comments = append(comments, rc.toModel())
	}

	return comments, nil
}

// refusalReason tells a switched-off comment system apart from a gated one, so
// the operator is not advised to authenticate against comments that do not
// exist. An unreadable body falls back to the general refusal.
func refusalReason(body []byte) error {
	var wpError struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(body, &wpError); err == nil && wpError.Code == restCommentDisabledCode {
		return ErrCommentsDisabled
	}

	return ErrCommentsNotAccessible
}

// toModel converts one REST comment into the exported shape.
func (rc restComment) toModel() models.WordPressComment {
	status := rc.Status
	if status == "" {
		status = defaultCommentStatus
	}

	return models.WordPressComment{
		ID:           rc.ID,
		Post:         rc.Post,
		Parent:       rc.Parent,
		Author:       rc.AuthorName,
		AuthorURL:    rc.AuthorURL,
		AuthorAvatar: largestAvatar(rc.AvatarURLs),
		Date:         rc.Date,
		DateGMT:      rc.DateGMT,
		Content:      rc.Content.Rendered,
		Status:       status,
		Type:         rc.Type,
		Link:         rc.Link,
	}
}

// largestAvatar picks the biggest gravatar rendition WordPress offers, keyed by
// pixel size ("24", "48", "96"). A target site scales it down; it cannot invent
// pixels the export did not carry.
func largestAvatar(urls map[string]string) string {
	best, bestSize := "", -1
	for size, u := range urls {
		if u == "" {
			continue
		}
		n := 0
		if _, err := fmt.Sscanf(size, "%d", &n); err != nil {
			continue
		}
		if n > bestSize {
			best, bestSize = u, n
		}
	}

	return best
}
