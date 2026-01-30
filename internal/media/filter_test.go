package media

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tradik/wpexporter/pkg/models"
)

func TestNewFilter(t *testing.T) {
	f := NewFilter()
	assert.NotNil(t, f)
}

func TestFilterRelevantMedia_FeaturedImages(t *testing.T) {
	posts := []models.WordPressPost{
		{ID: 1, FeaturedMedia: 100, Content: models.RenderedContent{Rendered: ""}},
		{ID: 2, FeaturedMedia: 200, Content: models.RenderedContent{Rendered: ""}},
	}
	pages := []models.WordPressPost{
		{ID: 3, FeaturedMedia: 300, Content: models.RenderedContent{Rendered: ""}},
	}
	allMedia := []models.WordPressMedia{
		{ID: 100, SourceURL: "https://example.com/img100.jpg"},
		{ID: 200, SourceURL: "https://example.com/img200.jpg"},
		{ID: 300, SourceURL: "https://example.com/img300.jpg"},
		{ID: 400, SourceURL: "https://example.com/img400.jpg"}, // Not referenced
	}

	f := NewFilter()
	filtered := f.FilterRelevantMedia(posts, pages, allMedia)

	assert.Len(t, filtered, 3)
	ids := make(map[int]bool)
	for _, m := range filtered {
		ids[m.ID] = true
	}
	assert.True(t, ids[100])
	assert.True(t, ids[200])
	assert.True(t, ids[300])
	assert.False(t, ids[400])
}

func TestFilterRelevantMedia_ContentImages(t *testing.T) {
	posts := []models.WordPressPost{
		{
			ID:            1,
			FeaturedMedia: 0,
			Content: models.RenderedContent{
				Rendered: `<p>Some text</p><img src="https://example.com/content-img.jpg" /><p>More text</p>`,
			},
		},
	}
	pages := []models.WordPressPost{}
	allMedia := []models.WordPressMedia{
		{ID: 100, SourceURL: "https://example.com/content-img.jpg"},
		{ID: 200, SourceURL: "https://example.com/other-img.jpg"},
	}

	f := NewFilter()
	filtered := f.FilterRelevantMedia(posts, pages, allMedia)

	assert.Len(t, filtered, 1)
	assert.Equal(t, 100, filtered[0].ID)
}

func TestFilterRelevantMedia_ExcerptImages(t *testing.T) {
	posts := []models.WordPressPost{
		{
			ID:            1,
			FeaturedMedia: 0,
			Content:       models.RenderedContent{Rendered: ""},
			Excerpt: models.RenderedContent{
				Rendered: `<img src="https://example.com/excerpt-img.jpg" />`,
			},
		},
	}
	pages := []models.WordPressPost{}
	allMedia := []models.WordPressMedia{
		{ID: 100, SourceURL: "https://example.com/excerpt-img.jpg"},
		{ID: 200, SourceURL: "https://example.com/other-img.jpg"},
	}

	f := NewFilter()
	filtered := f.FilterRelevantMedia(posts, pages, allMedia)

	assert.Len(t, filtered, 1)
	assert.Equal(t, 100, filtered[0].ID)
}

func TestFilterRelevantMedia_MediaSizes(t *testing.T) {
	posts := []models.WordPressPost{
		{
			ID:            1,
			FeaturedMedia: 0,
			Content: models.RenderedContent{
				Rendered: `<img src="https://example.com/img-300x200.jpg" />`,
			},
		},
	}
	pages := []models.WordPressPost{}
	allMedia := []models.WordPressMedia{
		{
			ID:        100,
			SourceURL: "https://example.com/img.jpg",
			MediaDetails: models.MediaDetails{
				Sizes: map[string]models.MediaSize{
					"medium": {SourceURL: "https://example.com/img-300x200.jpg"},
					"large":  {SourceURL: "https://example.com/img-1024x768.jpg"},
				},
			},
		},
	}

	f := NewFilter()
	filtered := f.FilterRelevantMedia(posts, pages, allMedia)

	assert.Len(t, filtered, 1)
	assert.Equal(t, 100, filtered[0].ID)
}

func TestFilterRelevantMedia_EmptyInputs(t *testing.T) {
	f := NewFilter()

	// Empty posts and pages
	filtered := f.FilterRelevantMedia(nil, nil, []models.WordPressMedia{{ID: 1}})
	assert.Len(t, filtered, 0)

	// Empty media
	filtered = f.FilterRelevantMedia(
		[]models.WordPressPost{{ID: 1, FeaturedMedia: 100}},
		nil,
		nil,
	)
	assert.Len(t, filtered, 0)
}

func TestFilterRelevantMedia_DuplicateReferences(t *testing.T) {
	posts := []models.WordPressPost{
		{ID: 1, FeaturedMedia: 100, Content: models.RenderedContent{Rendered: ""}},
		{ID: 2, FeaturedMedia: 100, Content: models.RenderedContent{Rendered: ""}}, // Same featured image
	}
	pages := []models.WordPressPost{}
	allMedia := []models.WordPressMedia{
		{ID: 100, SourceURL: "https://example.com/img100.jpg"},
	}

	f := NewFilter()
	filtered := f.FilterRelevantMedia(posts, pages, allMedia)

	// Should only include the media item once
	assert.Len(t, filtered, 1)
	assert.Equal(t, 100, filtered[0].ID)
}
