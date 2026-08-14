package export

// Reader comments in the markdown/ssg export (#35).
//
// Comments are structured records, not documents: nobody edits a comment in a
// text editor, and a target system inserts them into a table keyed by the page
// they belong to. So they leave the export as one comments.json next to
// metadata.json — addressed by URL, because a WordPress post ID means nothing
// on the other side of a migration.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tradik/wpexporter/pkg/models"
)

// commentsFileName is where the markdown export leaves the comment records.
const commentsFileName = "comments.json"

// commentsFile is the on-disk shape of comments.json. The counts are stated
// rather than left to be recomputed: a consumer reading only the header knows
// whether the file is worth walking.
type commentsFile struct {
	Total      int                       `json:"total"`
	Pages      int                       `json:"pages"`
	ExportedAt time.Time                 `json:"exported_at"`
	Comments   []models.WordPressComment `json:"comments"`
}

// exportComments writes comments.json into the export directory. An export
// without comments writes nothing — an empty file would read as "this site has
// no comments" when the truth may be "comments were not requested".
func (e *Exporter) exportComments(comments []models.WordPressComment) error {
	if len(comments) == 0 {
		return nil
	}

	payload := commentsFile{
		Total:      len(comments),
		Pages:      countCommentedPages(comments),
		ExportedAt: time.Now(),
		Comments:   comments,
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal comments: %w", err)
	}

	path := filepath.Join(e.config.Output, commentsFileName)

	return os.WriteFile(path, data, 0600)
}

// countCommentedPages counts the distinct pages the comments belong to.
func countCommentedPages(comments []models.WordPressComment) int {
	seen := make(map[string]struct{}, len(comments))
	for _, c := range comments {
		key := c.PostURL
		if key == "" {
			key = fmt.Sprintf("#%d", c.Post)
		}
		seen[key] = struct{}{}
	}

	return len(seen)
}

// resolveCommentAddresses fills every comment's PostURL from the post it
// belongs to, and drops the fragment from its own link.
//
// A comment's REST payload addresses its post by numeric ID only, and the
// #comment-123 fragment on its link is a permalink into the old theme. What
// survives a migration is the page address, so that is what the export states —
// in the same form as the post's own link, absolute or root-relative.
func (e *Exporter) resolveCommentAddresses(data *models.ExportData) {
	if len(data.Comments) == 0 {
		return
	}

	links := make(map[int]string, len(data.Posts)+len(data.Pages))
	collect := func(posts []models.WordPressPost) {
		for i := range posts {
			links[posts[i].ID] = posts[i].Link
		}
	}
	collect(data.Posts)
	collect(data.Pages)
	for t := range data.CustomTypes {
		collect(data.CustomTypes[t].Posts)
	}

	rootStyle := e.config.EffectiveLinkStyle() == "root"

	for i := range data.Comments {
		comment := &data.Comments[i]
		if rootStyle {
			comment.Link = e.rootRelativeURL(comment.Link)
		}

		if link, ok := links[comment.Post]; ok {
			comment.PostURL = link
			continue
		}

		// The commented post was not exported (excluded by --no-posts, a path
		// filter, or left in draft): fall back to the comment's own permalink,
		// which still names the page.
		comment.PostURL = stripFragment(comment.Link)
	}
}

// stripFragment removes a #comment-123 anchor, leaving the page address.
func stripFragment(rawURL string) string {
	for i := 0; i < len(rawURL); i++ {
		if rawURL[i] == '#' {
			return rawURL[:i]
		}
	}

	return rawURL
}
