package localdb

import (
	"database/sql"
	"fmt"
	"slices"

	"github.com/tradik/wpexporter/pkg/models"
)

// commentsQuery reads approved reader comments on a batch of documents. The
// e-mail address, IP and browser of the commenter are never selected.
const commentsQuery = "SELECT comment_ID, comment_post_ID, comment_parent, comment_author, comment_author_url, " +
	"comment_date, comment_date_gmt, comment_content FROM {comments} " +
	"WHERE comment_approved = '1' AND comment_type IN ('', 'comment') AND comment_post_ID IN (?) ORDER BY comment_ID"

// GetComments returns the approved comments on the exported documents. Their
// bodies go through the same cleaning as raw document content: comments are
// stored without paragraphs.
func (s *Source) GetComments() ([]models.WordPressComment, error) {
	links, err := s.documentLinks()
	if err != nil {
		return nil, err
	}

	docIDs := make([]int, 0, len(links))
	for id := range links {
		docIDs = append(docIDs, id)
	}
	slices.Sort(docIDs)

	comments := []models.WordPressComment{}
	for start := 0; start < len(docIDs); start += batchSize {
		chunk := docIDs[start:min(start+batchSize, len(docIDs))]
		err := s.q.each(s.ctx, func(r *sql.Rows) error {
			var c models.WordPressComment
			var date, dateGMT, content string
			if err := r.Scan(&c.ID, &c.Post, &c.Parent, &c.Author, &c.AuthorURL, &date, &dateGMT, &content); err != nil {
				return err
			}
			c.Date, c.DateGMT = wpTime(date, ""), wpTime(dateGMT, date)
			c.Content = CleanRawContent(content)
			c.Status, c.Type = "approved", "comment"
			c.PostURL = links[c.Post]
			c.Link = fmt.Sprintf("%s#comment-%d", c.PostURL, c.ID)
			comments = append(comments, c)
			return nil
		}, commentsQuery, list(chunk))
		if err != nil {
			return nil, s.fail(fmt.Errorf("reading comments: %w", err))
		}
	}
	return comments, nil
}

// documentLinks maps every exported document's ID to its address.
func (s *Source) documentLinks() (map[int]string, error) {
	types, err := s.documentTypes()
	if err != nil {
		return nil, err
	}
	links := map[int]string{}
	for _, t := range types {
		docs, err := s.documents(t)
		if err != nil {
			return nil, err
		}
		for _, d := range docs {
			links[d.ID] = d.Link
		}
	}
	return links, nil
}
