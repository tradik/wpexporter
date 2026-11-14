package localdb

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/tradik/wpexporter/internal/phpserialize"
	"github.com/tradik/wpexporter/pkg/models"
)

// batchSize is how many documents are read per query; their meta and terms
// follow in one query each per batch, never one per document.
const batchSize = 500

// published is the filter every document passes: what a visitor can read.
// Drafts, private entries, the trash, revisions and password-protected
// documents never leave the database.
const published = "post_status = 'publish' AND post_password = ''"

// docQuery reads one batch of one type's published documents.
const docQuery = "SELECT ID, post_author, post_date, post_date_gmt, post_content, post_title, post_excerpt, " +
	"comment_status, ping_status, post_name, post_modified, post_modified_gmt, post_parent, guid " +
	"FROM {posts} WHERE post_type = ? AND " + published + " AND ID > ? ORDER BY ID LIMIT ?"

// GetPosts returns the published blog posts.
func (s *Source) GetPosts() ([]models.WordPressPost, error) { return s.documents("post") }

// GetPages returns the published pages.
func (s *Source) GetPages() ([]models.WordPressPost, error) { return s.documents("page") }

// documents returns every published document of one type, read once.
func (s *Source) documents(postType string) ([]models.WordPressPost, error) {
	if docs, ok := s.docs[postType]; ok {
		return docs, nil
	}

	var all []models.WordPressPost
	for after := 0; ; {
		batch, err := s.documentBatch(postType, after)
		if err != nil {
			return nil, s.fail(fmt.Errorf("reading %s documents: %w", postType, err))
		}
		all = append(all, batch...)
		if len(batch) < batchSize {
			break
		}
		after = batch[len(batch)-1].ID
	}

	if err := s.applyLinks(postType, all); err != nil {
		return nil, err
	}
	s.docs[postType] = all
	return all, nil
}

// documentBatch reads up to batchSize documents with IDs above after, with
// their meta and terms.
func (s *Source) documentBatch(postType string, after int) ([]models.WordPressPost, error) {
	var batch []models.WordPressPost
	err := s.q.each(s.ctx, func(r *sql.Rows) error {
		var p models.WordPressPost
		var date, dateGMT, modified, modifiedGMT, content, excerpt, title, guid string
		if err := r.Scan(&p.ID, &p.Author, &date, &dateGMT, &content, &title, &excerpt,
			&p.CommentStatus, &p.PingStatus, &p.Slug, &modified, &modifiedGMT, &p.Parent, &guid); err != nil {
			return err
		}
		p.Type, p.Status = postType, "publish"
		p.Date, p.DateGMT = wpTime(date, ""), wpTime(dateGMT, date)
		p.Modified, p.ModifiedGMT = wpTime(modified, ""), wpTime(modifiedGMT, modified)
		p.Title.Rendered, p.GUID.Rendered = title, guid
		p.Content.Rendered = CleanRawContent(content)
		p.Excerpt.Rendered = CleanRawContent(excerpt)
		batch = append(batch, p)
		return nil
	}, docQuery, postType, after, batchSize)
	if err != nil || len(batch) == 0 {
		return batch, err
	}

	if err := s.attachMeta(batch); err != nil {
		return nil, err
	}
	return batch, s.attachTerms(batch, postType)
}

// metaQuery reads the meta an export carries: the featured image, the page
// template, and the public keys — the ones not starting with "_", which is
// what the REST API exposes too.
const metaQuery = "SELECT post_id, meta_key, meta_value FROM {postmeta} WHERE post_id IN (?) " +
	"AND (meta_key IN ('_thumbnail_id', '_wp_page_template') OR meta_key NOT LIKE ?) ORDER BY meta_id"

// attachMeta fills FeaturedMedia, Template, Sticky and Meta.
func (s *Source) attachMeta(batch []models.WordPressPost) error {
	index := indexByID(batch)
	err := s.q.each(s.ctx, func(r *sql.Rows) error {
		var id int
		var key string
		var value sql.NullString
		if err := r.Scan(&id, &key, &value); err != nil {
			return err
		}
		p := &batch[index[id]]
		switch key {
		case "_thumbnail_id":
			p.FeaturedMedia, _ = strconv.Atoi(value.String)
		case "_wp_page_template":
			if value.String != "default" {
				p.Template = value.String
			}
		default:
			if p.Meta == nil {
				p.Meta = map[string]interface{}{}
			}
			if _, seen := p.Meta[key]; !seen {
				p.Meta[key] = value.String
			}
		}
		return nil
	}, metaQuery, list(ids(batch)), `\_%`)
	if err != nil {
		return err
	}

	sticky, err := s.option("sticky_posts")
	if err != nil {
		return err
	}
	decoded, _ := phpserialize.Decode([]byte(sticky))
	for _, id := range phpserialize.Ints(decoded) {
		if i, ok := index[id]; ok {
			batch[i].Sticky = true
		}
	}
	return nil
}

// ids lists the documents' IDs.
func ids(docs []models.WordPressPost) []int {
	out := make([]int, len(docs))
	for i, d := range docs {
		out[i] = d.ID
	}
	return out
}

// indexByID maps each document's ID to its position.
func indexByID(docs []models.WordPressPost) map[int]int {
	out := make(map[int]int, len(docs))
	for i, d := range docs {
		out[d.ID] = i
	}
	return out
}

// wpTime reads a DATETIME column the way the REST API presents it. A zeroed
// GMT date (WordPress writes one for some imported entries) falls back to the
// local one.
func wpTime(value, fallback string) models.WordPressTime {
	if strings.HasPrefix(value, "0000-00-00") {
		value = fallback
	}
	t, err := time.Parse(time.DateTime, value)
	if err != nil {
		return models.WordPressTime{}
	}
	return models.WordPressTime{Time: t}
}
