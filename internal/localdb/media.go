package localdb

import (
	"database/sql"
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/tradik/wpexporter/internal/api"
	"github.com/tradik/wpexporter/internal/phpserialize"
	"github.com/tradik/wpexporter/pkg/models"
)

// mediaLibrary is the media library a visitor sees: attachments that are not
// attached to anything, or attached to a published document — what the REST
// API shows.
const mediaLibrary = "FROM {posts} a LEFT JOIN {posts} parent ON parent.ID = a.post_parent " +
	"WHERE a.post_type = 'attachment' AND a.post_status = 'inherit' AND (a.post_parent = 0 OR " +
	"(parent.post_status = 'publish' AND parent.post_password = ''))"

// mediaColumns are what a media read selects.
const mediaColumns = "SELECT a.ID, a.post_author, a.post_date, a.post_date_gmt, a.post_title, a.post_excerpt, " +
	"a.post_content, a.post_name, a.post_modified, a.post_modified_gmt, a.post_parent, a.post_mime_type, a.guid "

// mediaQuery reads one batch of the media library, by ID.
const mediaQuery = mediaColumns + mediaLibrary + " AND a.ID > ? ORDER BY a.ID LIMIT ?"

// mediaMetaQuery reads the three attachment meta keys an export needs.
const mediaMetaQuery = "SELECT post_id, meta_key, meta_value FROM {postmeta} WHERE post_id IN (?) AND meta_key IN " +
	"('_wp_attached_file', '_wp_attachment_image_alt', '_wp_attachment_metadata')"

// GetMedia returns the media library the export carries, with the addresses
// its files are served at — under Uploads().BaseURL, which is how the
// downloader knows to read them from disk instead.
func (s *Source) GetMedia() ([]models.WordPressMedia, error) {
	budget, proceed := s.opts.Limits.Take(api.CollectionMedia)
	if !proceed {
		return nil, nil
	}
	if budget > 0 {
		return s.newestMedia(budget)
	}

	var all []models.WordPressMedia
	for after := 0; ; {
		batch, err := s.mediaBatch(mediaQuery, after, batchSize)
		if err != nil {
			return nil, err
		}
		all = append(all, batch...)
		if len(batch) < batchSize {
			return all, nil
		}
		after = batch[len(batch)-1].ID
	}
}

// mediaBatch reads one batch of the library with query and args, with the
// attachment meta.
func (s *Source) mediaBatch(query string, args ...any) ([]models.WordPressMedia, error) {
	up, err := s.Uploads()
	if err != nil {
		return nil, err
	}
	home, _ := s.home()

	var batch []models.WordPressMedia
	err = s.q.each(func(r *sql.Rows) error {
		var m models.WordPressMedia
		var date, dateGMT, modified, modifiedGMT, title, caption, description, guid string
		if err := r.Scan(&m.ID, &m.Author, &date, &dateGMT, &title, &caption, &description, &m.Slug,
			&modified, &modifiedGMT, &m.Post, &m.MimeType, &guid); err != nil {
			return err
		}
		m.Date, m.DateGMT = wpTime(date, ""), wpTime(dateGMT, date)
		m.Modified, m.ModifiedGMT = wpTime(modified, ""), wpTime(modifiedGMT, modified)
		m.Title.Rendered, m.Caption.Rendered, m.Description.Rendered = title, caption, description
		m.GUID.Rendered, m.Type, m.Status = guid, "attachment", "inherit"
		m.MediaType = "file"
		if strings.HasPrefix(m.MimeType, "image/") {
			m.MediaType = "image"
		}
		m.Link = home + "/?attachment_id=" + strconv.Itoa(m.ID)
		batch = append(batch, m)
		return nil
	}, query, args...)
	if err == nil && len(batch) > 0 {
		err = s.attachMediaMeta(batch, up.BaseURL)
	}
	if err != nil {
		return nil, s.fail(fmt.Errorf("reading media: %w", err))
	}
	return batch, nil
}

// attachMediaMeta fills SourceURL, AltText and MediaDetails.
func (s *Source) attachMediaMeta(batch []models.WordPressMedia, baseURL string) error {
	index := make(map[int]int, len(batch))
	mediaIDs := make([]int, len(batch))
	for i, m := range batch {
		index[m.ID], mediaIDs[i] = i, m.ID
	}
	return s.q.each(func(r *sql.Rows) error {
		var id int
		var key string
		var value sql.NullString
		if err := r.Scan(&id, &key, &value); err != nil {
			return err
		}
		m := &batch[index[id]]
		switch key {
		case "_wp_attached_file":
			m.SourceURL = baseURL + "/" + strings.TrimLeft(value.String, "/")
		case "_wp_attachment_image_alt":
			m.AltText = value.String
		default:
			m.MediaDetails = mediaDetails(value.String, baseURL)
		}
		return nil
	}, mediaMetaQuery, list(mediaIDs))
}

// mediaDetails decodes _wp_attachment_metadata: dimensions, the file and its
// generated sizes. Undecodable metadata is simply absent.
func mediaDetails(serialized, baseURL string) models.MediaDetails {
	v, err := phpserialize.Decode([]byte(serialized))
	meta, ok := v.(phpserialize.Array)
	if err != nil || !ok {
		return models.MediaDetails{}
	}
	file, _ := meta.Get("file").(string)
	d := models.MediaDetails{Width: phpserialize.Int(meta.Get("width")), Height: phpserialize.Int(meta.Get("height")), File: file}

	sizes, _ := meta.Get("sizes").(phpserialize.Array)
	for _, e := range sizes {
		size, ok := e.Value.(phpserialize.Array)
		name, named := e.Key.(string)
		if !ok || !named {
			continue
		}
		sizeFile, _ := size.Get("file").(string)
		mime, _ := size.Get("mime-type").(string)
		if d.Sizes == nil {
			d.Sizes = map[string]models.MediaSize{}
		}
		d.Sizes[name] = models.MediaSize{
			File: sizeFile, Width: phpserialize.Int(size.Get("width")), Height: phpserialize.Int(size.Get("height")),
			MimeType: mime, SourceURL: baseURL + "/" + path.Join(path.Dir(file), sizeFile),
		}
	}
	return d
}
