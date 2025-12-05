package models

import (
	"encoding/json"
	"testing"
	"time"
)

func TestWordPressTimeUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Time
		hasError bool
	}{
		{
			name:     "WordPress format without timezone",
			input:    `"2024-01-15T10:30:00"`,
			expected: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
			hasError: false,
		},
		{
			name:     "ISO format with Z",
			input:    `"2024-01-15T10:30:00Z"`,
			expected: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
			hasError: false,
		},
		{
			name:     "ISO format with negative timezone offset",
			input:    `"2024-01-15T10:30:00-05:00"`,
			expected: time.Date(2024, 1, 15, 15, 30, 0, 0, time.UTC), // Should be converted to UTC
			hasError: false,
		},
		{
			name:     "ISO format with positive timezone offset",
			input:    `"2024-01-15T10:30:00+03:00"`,
			expected: time.Date(2024, 1, 15, 7, 30, 0, 0, time.UTC), // Should be converted to UTC
			hasError: false,
		},
		{
			name:     "RFC3339 format",
			input:    `"2024-01-15T10:30:00Z"`,
			expected: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
			hasError: false,
		},
		{
			name:     "RFC3339Nano format",
			input:    `"2024-01-15T10:30:00.123456789Z"`,
			expected: time.Date(2024, 1, 15, 10, 30, 0, 123456789, time.UTC),
			hasError: false,
		},
		{
			name:     "Invalid JSON",
			input:    `invalid json`,
			expected: time.Time{},
			hasError: true,
		},
		{
			name:     "Empty string",
			input:    `""`,
			expected: time.Time{}, // Should default to current time
			hasError: false,
		},
		{
			name:     "Invalid date format",
			input:    `"not-a-date"`,
			expected: time.Time{}, // Should default to current time
			hasError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var wt WordPressTime
			err := json.Unmarshal([]byte(tt.input), &wt)

			if (err != nil) != tt.hasError {
				t.Errorf("UnmarshalJSON() error = %v, hasError %v", err, tt.hasError)
				return
			}

			if !tt.hasError {
				// For cases that should default to current time, we can't compare exact time
				if tt.expected.IsZero() {
					if wt.IsZero() {
						t.Error("UnmarshalJSON() should set non-zero time for invalid dates")
					}
				} else {
					// Compare times with some tolerance for timezone conversions
					if !wt.Equal(tt.expected) {
						t.Errorf("UnmarshalJSON() time = %v, want %v", wt.Time, tt.expected)
					}
				}
			}
		})
	}
}

func TestWordPressTimeMarshalJSON(t *testing.T) {
	originalTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	wt := WordPressTime{Time: originalTime}

	data, err := json.Marshal(wt)
	if err != nil {
		t.Fatalf("MarshalJSON() error = %v", err)
	}

	var unmarshaled WordPressTime
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("UnmarshalJSON() error = %v", err)
	}

	if !unmarshaled.Equal(originalTime) {
		t.Errorf("Marshal/Unmarshal round trip failed: got %v, want %v", unmarshaled.Time, originalTime)
	}
}

func TestWordPressPostFields(t *testing.T) {
	postJSON := `{
		"id": 123,
		"date": "2024-01-15T10:30:00Z",
		"date_gmt": "2024-01-15T10:30:00Z",
		"guid": {"rendered": "https://example.com/?p=123"},
		"modified": "2024-01-15T10:30:00Z",
		"modified_gmt": "2024-01-15T10:30:00Z",
		"slug": "test-post",
		"status": "publish",
		"type": "post",
		"link": "https://example.com/test-post",
		"title": {"rendered": "Test Post"},
		"content": {"rendered": "Test content"},
		"excerpt": {"rendered": "Test excerpt"},
		"author": 1,
		"featured_media": 456,
		"comment_status": "open",
		"ping_status": "open",
		"sticky": false,
		"template": "",
		"format": "standard",
		"meta": {},
		"categories": [1, 2],
		"tags": [3, 4],
		"_links": {}
	}`

	var post WordPressPost
	err := json.Unmarshal([]byte(postJSON), &post)
	if err != nil {
		t.Fatalf("Failed to unmarshal WordPressPost: %v", err)
	}

	if post.ID != 123 {
		t.Errorf("WordPressPost ID = %d, want %d", post.ID, 123)
	}

	if post.Slug != "test-post" {
		t.Errorf("WordPressPost Slug = %s, want %s", post.Slug, "test-post")
	}

	if post.Status != "publish" {
		t.Errorf("WordPressPost Status = %s, want %s", post.Status, "publish")
	}

	if post.Type != "post" {
		t.Errorf("WordPressPost Type = %s, want %s", post.Type, "post")
	}

	if post.Title.Rendered != "Test Post" {
		t.Errorf("WordPressPost Title.Rendered = %s, want %s", post.Title.Rendered, "Test Post")
	}

	if post.Content.Rendered != "Test content" {
		t.Errorf("WordPressPost Content.Rendered = %s, want %s", post.Content.Rendered, "Test content")
	}

	if post.Author != 1 {
		t.Errorf("WordPressPost Author = %d, want %d", post.Author, 1)
	}

	if post.FeaturedMedia != 456 {
		t.Errorf("WordPressPost FeaturedMedia = %d, want %d", post.FeaturedMedia, 456)
	}

	if len(post.Categories) != 2 || post.Categories[0] != 1 || post.Categories[1] != 2 {
		t.Errorf("WordPressPost Categories = %v, want [1, 2]", post.Categories)
	}
}

func TestWordPressMediaFields(t *testing.T) {
	mediaJSON := `{
		"id": 789,
		"date": "2024-01-15T10:30:00Z",
		"date_gmt": "2024-01-15T10:30:00Z",
		"guid": {"rendered": "https://example.com/wp-content/uploads/2024/01/test.jpg"},
		"modified": "2024-01-15T10:30:00Z",
		"modified_gmt": "2024-01-15T10:30:00Z",
		"slug": "test-jpg",
		"status": "inherit",
		"type": "attachment",
		"link": "https://example.com/test-jpg/",
		"title": {"rendered": "Test Image"},
		"author": 1,
		"comment_status": "open",
		"ping_status": "closed",
		"template": "",
		"meta": {},
		"description": {"rendered": "Test description"},
		"caption": {"rendered": "Test caption"},
		"alt_text": "Test alt text",
		"media_type": "image",
		"mime_type": "image/jpeg",
		"media_details": {
			"width": 1920,
			"height": 1080,
			"file": "2024/01/test.jpg",
			"sizes": {
				"thumbnail": {
					"file": "test-150x150.jpg",
					"width": 150,
					"height": 150,
					"mime_type": "image/jpeg",
					"source_url": "https://example.com/wp-content/uploads/2024/01/test-150x150.jpg"
				}
			}
		},
		"post": 123,
		"source_url": "https://example.com/wp-content/uploads/2024/01/test.jpg",
		"_links": {}
	}`

	var media WordPressMedia
	err := json.Unmarshal([]byte(mediaJSON), &media)
	if err != nil {
		t.Fatalf("Failed to unmarshal WordPressMedia: %v", err)
	}

	if media.ID != 789 {
		t.Errorf("WordPressMedia ID = %d, want %d", media.ID, 789)
	}

	if media.Slug != "test-jpg" {
		t.Errorf("WordPressMedia Slug = %s, want %s", media.Slug, "test-jpg")
	}

	if media.AltText != "Test alt text" {
		t.Errorf("WordPressMedia AltText = %s, want %s", media.AltText, "Test alt text")
	}

	if media.MediaType != "image" {
		t.Errorf("WordPressMedia MediaType = %s, want %s", media.MediaType, "image")
	}

	if media.MimeType != "image/jpeg" {
		t.Errorf("WordPressMedia MimeType = %s, want %s", media.MimeType, "image/jpeg")
	}

	if media.SourceURL != "https://example.com/wp-content/uploads/2024/01/test.jpg" {
		t.Errorf("WordPressMedia SourceURL = %s, want %s", media.SourceURL, "https://example.com/wp-content/uploads/2024/01/test.jpg")
	}

	// Test media details
	if width, ok := media.MediaDetails.Width.(int64); ok && width != 1920 {
		t.Errorf("WordPressMedia MediaDetails.Width = %v, want %v", width, 1920)
	} else if _, ok := media.MediaDetails.Width.(int); ok && media.MediaDetails.Width != 1920 {
		t.Errorf("WordPressMedia MediaDetails.Width = %v, want %v", media.MediaDetails.Width, 1920)
	}

	if height, ok := media.MediaDetails.Height.(int64); ok && height != 1080 {
		t.Errorf("WordPressMedia MediaDetails.Height = %v, want %v", height, 1080)
	} else if _, ok := media.MediaDetails.Height.(int); ok && media.MediaDetails.Height != 1080 {
		t.Errorf("WordPressMedia MediaDetails.Height = %v, want %v", media.MediaDetails.Height, 1080)
	}

	// Test media sizes
	thumbnail, exists := media.MediaDetails.Sizes["thumbnail"]
	if !exists {
		t.Fatal("WordPressMedia MediaDetails.Sizes should contain thumbnail")
	}

	if thumbnailWidth, ok := thumbnail.Width.(int64); ok && thumbnailWidth != 150 {
		t.Errorf("WordPressMedia thumbnail width = %v, want %v", thumbnailWidth, 150)
	} else if _, ok := thumbnail.Width.(int); ok && thumbnail.Width != 150 {
		t.Errorf("WordPressMedia thumbnail width = %v, want %v", thumbnail.Width, 150)
	}
}

func TestWordPressCategoryFields(t *testing.T) {
	categoryJSON := `{
		"id": 5,
		"count": 25,
		"description": "A test category",
		"link": "https://example.com/category/test/",
		"name": "Test Category",
		"slug": "test-category",
		"taxonomy": "category",
		"parent": 0,
		"meta": [],
		"_links": {}
	}`

	var category WordPressCategory
	err := json.Unmarshal([]byte(categoryJSON), &category)
	if err != nil {
		t.Fatalf("Failed to unmarshal WordPressCategory: %v", err)
	}

	if category.ID != 5 {
		t.Errorf("WordPressCategory ID = %d, want %d", category.ID, 5)
	}

	if category.Name != "Test Category" {
		t.Errorf("WordPressCategory Name = %s, want %s", category.Name, "Test Category")
	}

	if category.Slug != "test-category" {
		t.Errorf("WordPressCategory Slug = %s, want %s", category.Slug, "test-category")
	}

	if category.Count != 25 {
		t.Errorf("WordPressCategory Count = %d, want %d", category.Count, 25)
	}

	if category.Taxonomy != "category" {
		t.Errorf("WordPressCategory Taxonomy = %s, want %s", category.Taxonomy, "category")
	}
}

func TestWordPressTagFields(t *testing.T) {
	tagJSON := `{
		"id": 10,
		"count": 15,
		"description": "A test tag",
		"link": "https://example.com/tag/test/",
		"name": "Test Tag",
		"slug": "test-tag",
		"taxonomy": "post_tag",
		"meta": [],
		"_links": {}
	}`

	var tag WordPressTag
	err := json.Unmarshal([]byte(tagJSON), &tag)
	if err != nil {
		t.Fatalf("Failed to unmarshal WordPressTag: %v", err)
	}

	if tag.ID != 10 {
		t.Errorf("WordPressTag ID = %d, want %d", tag.ID, 10)
	}

	if tag.Name != "Test Tag" {
		t.Errorf("WordPressTag Name = %s, want %s", tag.Name, "Test Tag")
	}

	if tag.Slug != "test-tag" {
		t.Errorf("WordPressTag Slug = %s, want %s", tag.Slug, "test-tag")
	}

	if tag.Count != 15 {
		t.Errorf("WordPressTag Count = %d, want %d", tag.Count, 15)
	}

	if tag.Taxonomy != "post_tag" {
		t.Errorf("WordPressTag Taxonomy = %s, want %s", tag.Taxonomy, "post_tag")
	}
}

func TestWordPressUserFields(t *testing.T) {
	userJSON := `{
		"id": 1,
		"name": "Admin User",
		"url": "https://admin.example.com",
		"description": "Site administrator",
		"link": "https://example.com/author/admin/",
		"slug": "admin",
		"avatar_urls": {
			"24": "https://example.com/wp-content/uploads/2024/01/avatar-24x24.jpg",
			"48": "https://example.com/wp-content/uploads/2024/01/avatar-48x48.jpg",
			"96": "https://example.com/wp-content/uploads/2024/01/avatar-96x96.jpg"
		},
		"meta": [],
		"_links": {}
	}`

	var user WordPressUser
	err := json.Unmarshal([]byte(userJSON), &user)
	if err != nil {
		t.Fatalf("Failed to unmarshal WordPressUser: %v", err)
	}

	if user.ID != 1 {
		t.Errorf("WordPressUser ID = %d, want %d", user.ID, 1)
	}

	if user.Name != "Admin User" {
		t.Errorf("WordPressUser Name = %s, want %s", user.Name, "Admin User")
	}

	if user.Slug != "admin" {
		t.Errorf("WordPressUser Slug = %s, want %s", user.Slug, "admin")
	}

	if user.URL != "https://admin.example.com" {
		t.Errorf("WordPressUser URL = %s, want %s", user.URL, "https://admin.example.com")
	}

	if user.Description != "Site administrator" {
		t.Errorf("WordPressUser Description = %s, want %s", user.Description, "Site administrator")
	}

	// Test avatar URLs
	if len(user.AvatarURLs) != 3 {
		t.Errorf("WordPressUser AvatarURLs length = %d, want %d", len(user.AvatarURLs), 3)
	}

	if user.AvatarURLs["24"] != "https://example.com/wp-content/uploads/2024/01/avatar-24x24.jpg" {
		t.Errorf("WordPressUser AvatarURLs[24] = %s, want %s", user.AvatarURLs["24"], "https://example.com/wp-content/uploads/2024/01/avatar-24x24.jpg")
	}
}

func TestExportDataStructure(t *testing.T) {
	exportJSON := `{
		"site": {
			"name": "Test Site",
			"description": "Test Description",
			"url": "https://example.com",
			"home_url": "https://example.com",
			"admin_email": "admin@example.com",
			"timezone": "UTC",
			"date_format": "Y-m-d",
			"time_format": "H:i:s",
			"start_of_week": 1,
			"language": "en_US"
		},
		"posts": [
			{
				"id": 1,
				"slug": "test-post",
				"title": {"rendered": "Test Post"},
				"content": {"rendered": "Test content"},
				"status": "publish",
				"type": "post",
				"date": "2024-01-15T10:30:00Z",
				"date_gmt": "2024-01-15T10:30:00Z",
				"modified": "2024-01-15T10:30:00Z",
				"modified_gmt": "2024-01-15T10:30:00Z",
				"link": "https://example.com/test-post",
				"author": 1,
				"featured_media": 0,
				"comment_status": "open",
				"ping_status": "open",
				"sticky": false,
				"template": "",
				"format": "standard",
				"meta": {},
				"categories": [],
				"tags": [],
				"_links": {}
			}
		],
		"pages": [],
		"media": [],
		"categories": [],
		"tags": [],
		"users": [],
		"exported_at": "2024-01-15T10:30:00Z",
		"stats": {
			"total_posts": 1,
			"total_pages": 0,
			"total_media": 0,
			"total_categories": 0,
			"total_tags": 0,
			"total_users": 0,
			"media_downloaded": 0,
			"brute_force_found": 0
		}
	}`

	var exportData ExportData
	err := json.Unmarshal([]byte(exportJSON), &exportData)
	if err != nil {
		t.Fatalf("Failed to unmarshal ExportData: %v", err)
	}

	if exportData.Site.Name != "Test Site" {
		t.Errorf("ExportData Site.Name = %s, want %s", exportData.Site.Name, "Test Site")
	}

	if len(exportData.Posts) != 1 {
		t.Errorf("ExportData Posts length = %d, want %d", len(exportData.Posts), 1)
	}

	if exportData.Stats.TotalPosts != 1 {
		t.Errorf("ExportData Stats.TotalPosts = %d, want %d", exportData.Stats.TotalPosts, 1)
	}

	if exportData.Stats.TotalPages != 0 {
		t.Errorf("ExportData Stats.TotalPages = %d, want %d", exportData.Stats.TotalPages, 0)
	}
}

func TestRenderedContent(t *testing.T) {
	contentJSON := `{
		"rendered": "<p>Test content</p>",
		"protected": false
	}`

	var content RenderedContent
	err := json.Unmarshal([]byte(contentJSON), &content)
	if err != nil {
		t.Fatalf("Failed to unmarshal RenderedContent: %v", err)
	}

	if content.Rendered != "<p>Test content</p>" {
		t.Errorf("RenderedContent.Rendered = %s, want %s", content.Rendered, "<p>Test content</p>")
	}

	if content.Protected != false {
		t.Errorf("RenderedContent.Protected = %v, want %v", content.Protected, false)
	}
}

func TestLinksStructure(t *testing.T) {
	linksJSON := `{
		"self": [{"href": "https://example.com/wp-json/wp/v2/posts/1"}],
		"collection": [{"href": "https://example.com/wp-json/wp/v2/posts"}],
		"about": [{"href": "https://example.com/wp-json/wp/v2/types/post"}],
		"author": [{"href": "https://example.com/wp-json/wp/v2/users/1"}],
		"replies": [{"href": "https://example.com/wp-json/wp/v2/comments"}],
		"version-history": [{"href": "https://example.com/wp-json/wp/v2/posts/1/revisions"}],
		"wp:featuredmedia": [{"href": "https://example.com/wp-json/wp/v2/media/456"}],
		"wp:attachment": [{"href": "https://example.com/wp-json/wp/v2/media"}],
		"wp:term": [{"href": "https://example.com/wp-json/wp/v2/categories"}],
		"curies": [{"name": "wp", "href": "https://api.w.org/{rel}", "templated": true}]
	}`

	var links Links
	err := json.Unmarshal([]byte(linksJSON), &links)
	if err != nil {
		t.Fatalf("Failed to unmarshal Links: %v", err)
	}

	if len(links.Self) != 1 {
		t.Errorf("Links.Self length = %d, want %d", len(links.Self), 1)
	}

	if links.Self[0].Href != "https://example.com/wp-json/wp/v2/posts/1" {
		t.Errorf("Links.Self[0].Href = %s, want %s", links.Self[0].Href, "https://example.com/wp-json/wp/v2/posts/1")
	}

	if len(links.Collection) != 1 {
		t.Errorf("Links.Collection length = %d, want %d", len(links.Collection), 1)
	}
}

func TestGUIDStructure(t *testing.T) {
	guidJSON := `{
		"rendered": "https://example.com/?p=123"
	}`

	var guid GUID
	err := json.Unmarshal([]byte(guidJSON), &guid)
	if err != nil {
		t.Fatalf("Failed to unmarshal GUID: %v", err)
	}

	if guid.Rendered != "https://example.com/?p=123" {
		t.Errorf("GUID.Rendered = %s, want %s", guid.Rendered, "https://example.com/?p=123")
	}
}

func TestMediaSizeStructure(t *testing.T) {
	mediaSizeJSON := `{
		"file": "test-150x150.jpg",
		"width": 150,
		"height": 150,
		"mime_type": "image/jpeg",
		"source_url": "https://example.com/wp-content/uploads/2024/01/test-150x150.jpg"
	}`

	var mediaSize MediaSize
	err := json.Unmarshal([]byte(mediaSizeJSON), &mediaSize)
	if err != nil {
		t.Fatalf("Failed to unmarshal MediaSize: %v", err)
	}

	if mediaSize.File != "test-150x150.jpg" {
		t.Errorf("MediaSize.File = %s, want %s", mediaSize.File, "test-150x150.jpg")
	}

	if width, ok := mediaSize.Width.(int64); ok && width != 150 {
		t.Errorf("MediaSize.Width = %v, want %v", width, 150)
	} else if _, ok := mediaSize.Width.(int); ok && mediaSize.Width != 150 {
		t.Errorf("MediaSize.Width = %v, want %v", mediaSize.Width, 150)
	}

	if height, ok := mediaSize.Height.(int64); ok && height != 150 {
		t.Errorf("MediaSize.Height = %v, want %v", height, 150)
	} else if _, ok := mediaSize.Height.(int); ok && mediaSize.Height != 150 {
		t.Errorf("MediaSize.Height = %v, want %v", mediaSize.Height, 150)
	}

	if mediaSize.MimeType != "image/jpeg" {
		t.Errorf("MediaSize.MimeType = %s, want %s", mediaSize.MimeType, "image/jpeg")
	}
}
