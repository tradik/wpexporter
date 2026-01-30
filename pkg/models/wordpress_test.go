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

// TestWordPressTimeUnmarshalJSON_DirectCall tests the UnmarshalJSON method directly
// to cover error paths that can't be reached via json.Unmarshal wrapper
func TestWordPressTimeUnmarshalJSON_DirectCall(t *testing.T) {
	// Test with non-string JSON (number) - should return error
	var wt WordPressTime
	err := wt.UnmarshalJSON([]byte("123"))
	if err == nil {
		t.Error("UnmarshalJSON should return error for non-string JSON")
	}

	// Test with JSON array - should return error
	err = wt.UnmarshalJSON([]byte("[1,2,3]"))
	if err == nil {
		t.Error("UnmarshalJSON should return error for JSON array")
	}

	// Test with JSON object - should return error
	err = wt.UnmarshalJSON([]byte(`{"key": "value"}`))
	if err == nil {
		t.Error("UnmarshalJSON should return error for JSON object")
	}

	// Test with JSON boolean - should return error
	err = wt.UnmarshalJSON([]byte("true"))
	if err == nil {
		t.Error("UnmarshalJSON should return error for JSON boolean")
	}

	// Test with null JSON - should NOT error (Go unmarshals null to empty string)
	// The function should handle this gracefully and set time to Now()
	err = wt.UnmarshalJSON([]byte("null"))
	if err != nil {
		t.Errorf("UnmarshalJSON should not error for null JSON: %v", err)
	}
	// Time should be set to approximately now (within last second)
	if wt.IsZero() {
		t.Error("UnmarshalJSON should set non-zero time for null JSON")
	}

	// Test with invalid JSON syntax
	err = wt.UnmarshalJSON([]byte("{invalid"))
	if err == nil {
		t.Error("UnmarshalJSON should return error for invalid JSON syntax")
	}
}

func TestWooCommerceProductFields(t *testing.T) {
	productJSON := `{
		"id": 100,
		"name": "Test Product",
		"slug": "test-product",
		"permalink": "https://example.com/product/test-product",
		"date_created": "2024-01-15T10:30:00",
		"date_modified": "2024-01-15T12:00:00",
		"type": "simple",
		"status": "publish",
		"featured": true,
		"catalog_visibility": "visible",
		"description": "<p>Product description</p>",
		"short_description": "<p>Short desc</p>",
		"sku": "TEST-001",
		"price": "29.99",
		"regular_price": "39.99",
		"sale_price": "29.99",
		"on_sale": true,
		"purchasable": true,
		"total_sales": 150,
		"virtual": false,
		"downloadable": false,
		"tax_status": "taxable",
		"tax_class": "",
		"manage_stock": true,
		"stock_quantity": 50,
		"stock_status": "instock",
		"backorders": "no",
		"backorders_allowed": false,
		"backordered": false,
		"sold_individually": false,
		"weight": "1.5",
		"dimensions": {
			"length": "10",
			"width": "5",
			"height": "2"
		},
		"shipping_required": true,
		"shipping_taxable": true,
		"shipping_class": "",
		"shipping_class_id": 0,
		"reviews_allowed": true,
		"average_rating": "4.5",
		"rating_count": 25,
		"parent_id": 0,
		"purchase_note": "Thank you for your purchase",
		"categories": [
			{"id": 1, "name": "Electronics", "slug": "electronics"}
		],
		"tags": [
			{"id": 10, "name": "Sale", "slug": "sale"}
		],
		"images": [
			{
				"id": 50,
				"src": "https://example.com/img1.jpg",
				"name": "Product Image",
				"alt": "Product Alt Text",
				"date_created": "2024-01-15T10:30:00",
				"date_modified": "2024-01-15T10:30:00"
			}
		],
		"attributes": [
			{
				"id": 1,
				"name": "Color",
				"position": 0,
				"visible": true,
				"variation": true,
				"options": ["Red", "Blue", "Green"]
			}
		],
		"default_attributes": [],
		"variations": [101, 102],
		"meta_data": [
			{"id": 1, "key": "_custom_field", "value": "custom_value"}
		]
	}`

	var product WooCommerceProduct
	err := json.Unmarshal([]byte(productJSON), &product)
	if err != nil {
		t.Fatalf("Failed to unmarshal WooCommerceProduct: %v", err)
	}

	if product.ID != 100 {
		t.Errorf("WooCommerceProduct ID = %d, want %d", product.ID, 100)
	}

	if product.Name != "Test Product" {
		t.Errorf("WooCommerceProduct Name = %s, want %s", product.Name, "Test Product")
	}

	if product.SKU != "TEST-001" {
		t.Errorf("WooCommerceProduct SKU = %s, want %s", product.SKU, "TEST-001")
	}

	if product.Price != "29.99" {
		t.Errorf("WooCommerceProduct Price = %s, want %s", product.Price, "29.99")
	}

	if !product.OnSale {
		t.Error("WooCommerceProduct OnSale should be true")
	}

	if !product.Featured {
		t.Error("WooCommerceProduct Featured should be true")
	}

	if product.TotalSales != 150 {
		t.Errorf("WooCommerceProduct TotalSales = %d, want %d", product.TotalSales, 150)
	}

	if len(product.Categories) != 1 || product.Categories[0].Name != "Electronics" {
		t.Errorf("WooCommerceProduct Categories = %v, want [Electronics]", product.Categories)
	}

	if len(product.Images) != 1 || product.Images[0].Src != "https://example.com/img1.jpg" {
		t.Errorf("WooCommerceProduct Images = %v", product.Images)
	}

	if len(product.Attributes) != 1 || product.Attributes[0].Name != "Color" {
		t.Errorf("WooCommerceProduct Attributes = %v", product.Attributes)
	}

	if len(product.Variations) != 2 {
		t.Errorf("WooCommerceProduct Variations = %v, want [101, 102]", product.Variations)
	}

	// Test dimensions
	if product.Dimensions.Length != "10" {
		t.Errorf("WooCommerceProduct Dimensions.Length = %s, want %s", product.Dimensions.Length, "10")
	}
}

func TestProductDimensionsFields(t *testing.T) {
	dimensionsJSON := `{
		"length": "25.5",
		"width": "15.0",
		"height": "10.0"
	}`

	var dimensions ProductDimensions
	err := json.Unmarshal([]byte(dimensionsJSON), &dimensions)
	if err != nil {
		t.Fatalf("Failed to unmarshal ProductDimensions: %v", err)
	}

	if dimensions.Length != "25.5" {
		t.Errorf("ProductDimensions Length = %s, want %s", dimensions.Length, "25.5")
	}

	if dimensions.Width != "15.0" {
		t.Errorf("ProductDimensions Width = %s, want %s", dimensions.Width, "15.0")
	}

	if dimensions.Height != "10.0" {
		t.Errorf("ProductDimensions Height = %s, want %s", dimensions.Height, "10.0")
	}
}

func TestProductCategoryFields(t *testing.T) {
	categoryJSON := `{
		"id": 5,
		"name": "Clothing",
		"slug": "clothing"
	}`

	var category ProductCategory
	err := json.Unmarshal([]byte(categoryJSON), &category)
	if err != nil {
		t.Fatalf("Failed to unmarshal ProductCategory: %v", err)
	}

	if category.ID != 5 {
		t.Errorf("ProductCategory ID = %d, want %d", category.ID, 5)
	}

	if category.Name != "Clothing" {
		t.Errorf("ProductCategory Name = %s, want %s", category.Name, "Clothing")
	}

	if category.Slug != "clothing" {
		t.Errorf("ProductCategory Slug = %s, want %s", category.Slug, "clothing")
	}
}

func TestProductTagFields(t *testing.T) {
	tagJSON := `{
		"id": 15,
		"name": "New Arrival",
		"slug": "new-arrival"
	}`

	var tag ProductTag
	err := json.Unmarshal([]byte(tagJSON), &tag)
	if err != nil {
		t.Fatalf("Failed to unmarshal ProductTag: %v", err)
	}

	if tag.ID != 15 {
		t.Errorf("ProductTag ID = %d, want %d", tag.ID, 15)
	}

	if tag.Name != "New Arrival" {
		t.Errorf("ProductTag Name = %s, want %s", tag.Name, "New Arrival")
	}

	if tag.Slug != "new-arrival" {
		t.Errorf("ProductTag Slug = %s, want %s", tag.Slug, "new-arrival")
	}
}

func TestProductImageFields(t *testing.T) {
	imageJSON := `{
		"id": 200,
		"src": "https://example.com/wp-content/uploads/product.jpg",
		"name": "product.jpg",
		"alt": "Product main image",
		"date_created": "2024-01-15T10:30:00",
		"date_modified": "2024-01-15T12:00:00"
	}`

	var image ProductImage
	err := json.Unmarshal([]byte(imageJSON), &image)
	if err != nil {
		t.Fatalf("Failed to unmarshal ProductImage: %v", err)
	}

	if image.ID != 200 {
		t.Errorf("ProductImage ID = %d, want %d", image.ID, 200)
	}

	if image.Src != "https://example.com/wp-content/uploads/product.jpg" {
		t.Errorf("ProductImage Src = %s, want %s", image.Src, "https://example.com/wp-content/uploads/product.jpg")
	}

	if image.Name != "product.jpg" {
		t.Errorf("ProductImage Name = %s, want %s", image.Name, "product.jpg")
	}

	if image.Alt != "Product main image" {
		t.Errorf("ProductImage Alt = %s, want %s", image.Alt, "Product main image")
	}
}

func TestProductAttributeFields(t *testing.T) {
	attributeJSON := `{
		"id": 3,
		"name": "Size",
		"position": 1,
		"visible": true,
		"variation": true,
		"options": ["Small", "Medium", "Large", "XL"]
	}`

	var attribute ProductAttribute
	err := json.Unmarshal([]byte(attributeJSON), &attribute)
	if err != nil {
		t.Fatalf("Failed to unmarshal ProductAttribute: %v", err)
	}

	if attribute.ID != 3 {
		t.Errorf("ProductAttribute ID = %d, want %d", attribute.ID, 3)
	}

	if attribute.Name != "Size" {
		t.Errorf("ProductAttribute Name = %s, want %s", attribute.Name, "Size")
	}

	if attribute.Position != 1 {
		t.Errorf("ProductAttribute Position = %d, want %d", attribute.Position, 1)
	}

	if !attribute.Visible {
		t.Error("ProductAttribute Visible should be true")
	}

	if !attribute.Variation {
		t.Error("ProductAttribute Variation should be true")
	}

	if len(attribute.Options) != 4 {
		t.Errorf("ProductAttribute Options length = %d, want %d", len(attribute.Options), 4)
	}
}

func TestProductMetaFields(t *testing.T) {
	metaJSON := `{
		"id": 50,
		"key": "_product_custom_field",
		"value": "custom value"
	}`

	var meta ProductMeta
	err := json.Unmarshal([]byte(metaJSON), &meta)
	if err != nil {
		t.Fatalf("Failed to unmarshal ProductMeta: %v", err)
	}

	if meta.ID != 50 {
		t.Errorf("ProductMeta ID = %d, want %d", meta.ID, 50)
	}

	if meta.Key != "_product_custom_field" {
		t.Errorf("ProductMeta Key = %s, want %s", meta.Key, "_product_custom_field")
	}

	if meta.Value != "custom value" {
		t.Errorf("ProductMeta Value = %v, want %s", meta.Value, "custom value")
	}
}

func TestSEODataFields(t *testing.T) {
	seoJSON := `{
		"seo_title": "Test SEO Title | Site Name",
		"meta_description": "This is the meta description for SEO",
		"meta_keywords": "test, keywords, seo",
		"og_title": "Open Graph Title",
		"og_description": "Open Graph Description",
		"og_image": "https://example.com/og-image.jpg",
		"canonical_url": "https://example.com/canonical-page"
	}`

	var seo SEOData
	err := json.Unmarshal([]byte(seoJSON), &seo)
	if err != nil {
		t.Fatalf("Failed to unmarshal SEOData: %v", err)
	}

	if seo.Title != "Test SEO Title | Site Name" {
		t.Errorf("SEOData Title = %s, want %s", seo.Title, "Test SEO Title | Site Name")
	}

	if seo.MetaDescription != "This is the meta description for SEO" {
		t.Errorf("SEOData MetaDescription = %s", seo.MetaDescription)
	}

	if seo.MetaKeywords != "test, keywords, seo" {
		t.Errorf("SEOData MetaKeywords = %s", seo.MetaKeywords)
	}

	if seo.OGTitle != "Open Graph Title" {
		t.Errorf("SEOData OGTitle = %s", seo.OGTitle)
	}

	if seo.OGDescription != "Open Graph Description" {
		t.Errorf("SEOData OGDescription = %s", seo.OGDescription)
	}

	if seo.OGImage != "https://example.com/og-image.jpg" {
		t.Errorf("SEOData OGImage = %s", seo.OGImage)
	}

	if seo.CanonicalURL != "https://example.com/canonical-page" {
		t.Errorf("SEOData CanonicalURL = %s", seo.CanonicalURL)
	}
}

func TestExportStatsFields(t *testing.T) {
	statsJSON := `{
		"total_posts": 100,
		"total_pages": 25,
		"total_products": 50,
		"total_media": 200,
		"total_categories": 10,
		"total_tags": 30,
		"total_users": 5,
		"media_downloaded": 180,
		"brute_force_found": 15
	}`

	var stats ExportStats
	err := json.Unmarshal([]byte(statsJSON), &stats)
	if err != nil {
		t.Fatalf("Failed to unmarshal ExportStats: %v", err)
	}

	if stats.TotalPosts != 100 {
		t.Errorf("ExportStats TotalPosts = %d, want %d", stats.TotalPosts, 100)
	}

	if stats.TotalPages != 25 {
		t.Errorf("ExportStats TotalPages = %d, want %d", stats.TotalPages, 25)
	}

	if stats.TotalProducts != 50 {
		t.Errorf("ExportStats TotalProducts = %d, want %d", stats.TotalProducts, 50)
	}

	if stats.TotalMedia != 200 {
		t.Errorf("ExportStats TotalMedia = %d, want %d", stats.TotalMedia, 200)
	}

	if stats.TotalCategories != 10 {
		t.Errorf("ExportStats TotalCategories = %d, want %d", stats.TotalCategories, 10)
	}

	if stats.TotalTags != 30 {
		t.Errorf("ExportStats TotalTags = %d, want %d", stats.TotalTags, 30)
	}

	if stats.TotalUsers != 5 {
		t.Errorf("ExportStats TotalUsers = %d, want %d", stats.TotalUsers, 5)
	}

	if stats.MediaDownloaded != 180 {
		t.Errorf("ExportStats MediaDownloaded = %d, want %d", stats.MediaDownloaded, 180)
	}

	if stats.BruteForceFound != 15 {
		t.Errorf("ExportStats BruteForceFound = %d, want %d", stats.BruteForceFound, 15)
	}
}

func TestSiteInfoFields(t *testing.T) {
	siteJSON := `{
		"name": "My WordPress Site",
		"description": "A great site description",
		"url": "https://example.com",
		"home_url": "https://example.com",
		"admin_email": "admin@example.com",
		"timezone": "Europe/London",
		"date_format": "F j, Y",
		"time_format": "g:i a",
		"start_of_week": 0,
		"language": "en_GB"
	}`

	var site SiteInfo
	err := json.Unmarshal([]byte(siteJSON), &site)
	if err != nil {
		t.Fatalf("Failed to unmarshal SiteInfo: %v", err)
	}

	if site.Name != "My WordPress Site" {
		t.Errorf("SiteInfo Name = %s, want %s", site.Name, "My WordPress Site")
	}

	if site.Description != "A great site description" {
		t.Errorf("SiteInfo Description = %s", site.Description)
	}

	if site.URL != "https://example.com" {
		t.Errorf("SiteInfo URL = %s", site.URL)
	}

	if site.AdminEmail != "admin@example.com" {
		t.Errorf("SiteInfo AdminEmail = %s", site.AdminEmail)
	}

	if site.Timezone != "Europe/London" {
		t.Errorf("SiteInfo Timezone = %s", site.Timezone)
	}

	if site.Language != "en_GB" {
		t.Errorf("SiteInfo Language = %s", site.Language)
	}

	if site.StartOfWeek != 0 {
		t.Errorf("SiteInfo StartOfWeek = %d, want %d", site.StartOfWeek, 0)
	}
}

func TestMediaDetailsFields(t *testing.T) {
	detailsJSON := `{
		"width": 1920,
		"height": 1080,
		"file": "2024/01/image.jpg",
		"sizes": {
			"thumbnail": {
				"file": "image-150x150.jpg",
				"width": 150,
				"height": 150,
				"mime_type": "image/jpeg",
				"source_url": "https://example.com/wp-content/uploads/2024/01/image-150x150.jpg"
			},
			"medium": {
				"file": "image-300x200.jpg",
				"width": 300,
				"height": 200,
				"mime_type": "image/jpeg",
				"source_url": "https://example.com/wp-content/uploads/2024/01/image-300x200.jpg"
			}
		},
		"image_meta": {
			"aperture": "2.8",
			"camera": "Canon EOS",
			"created_timestamp": "1705311000"
		},
		"length": 0,
		"filesize": 256000
	}`

	var details MediaDetails
	err := json.Unmarshal([]byte(detailsJSON), &details)
	if err != nil {
		t.Fatalf("Failed to unmarshal MediaDetails: %v", err)
	}

	if details.File != "2024/01/image.jpg" {
		t.Errorf("MediaDetails File = %s, want %s", details.File, "2024/01/image.jpg")
	}

	if len(details.Sizes) != 2 {
		t.Errorf("MediaDetails Sizes length = %d, want %d", len(details.Sizes), 2)
	}

	thumbnail, ok := details.Sizes["thumbnail"]
	if !ok {
		t.Fatal("MediaDetails Sizes should contain thumbnail")
	}

	if thumbnail.File != "image-150x150.jpg" {
		t.Errorf("MediaDetails thumbnail File = %s", thumbnail.File)
	}

	if details.ImageMeta == nil {
		t.Error("MediaDetails ImageMeta should not be nil")
	}
}

func TestLinkFields(t *testing.T) {
	linkJSON := `{
		"href": "https://example.com/wp-json/wp/v2/posts/1"
	}`

	var link Link
	err := json.Unmarshal([]byte(linkJSON), &link)
	if err != nil {
		t.Fatalf("Failed to unmarshal Link: %v", err)
	}

	if link.Href != "https://example.com/wp-json/wp/v2/posts/1" {
		t.Errorf("Link Href = %s", link.Href)
	}
}

func TestExportDataWithProducts(t *testing.T) {
	exportJSON := `{
		"site": {"name": "Test Shop"},
		"posts": [],
		"pages": [],
		"products": [
			{
				"id": 1,
				"name": "Test Product",
				"slug": "test-product",
				"type": "simple",
				"status": "publish",
				"price": "19.99",
				"date_created": "2024-01-15T10:30:00",
				"date_modified": "2024-01-15T10:30:00"
			}
		],
		"media": [],
		"categories": [],
		"tags": [],
		"users": [],
		"exported_at": "2024-01-15T10:30:00Z",
		"stats": {
			"total_posts": 0,
			"total_pages": 0,
			"total_products": 1,
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
		t.Fatalf("Failed to unmarshal ExportData with products: %v", err)
	}

	if len(exportData.Products) != 1 {
		t.Errorf("ExportData Products length = %d, want %d", len(exportData.Products), 1)
	}

	if exportData.Products[0].Name != "Test Product" {
		t.Errorf("ExportData Products[0].Name = %s, want %s", exportData.Products[0].Name, "Test Product")
	}

	if exportData.Stats.TotalProducts != 1 {
		t.Errorf("ExportData Stats.TotalProducts = %d, want %d", exportData.Stats.TotalProducts, 1)
	}
}

func TestWordPressPostWithSEO(t *testing.T) {
	postJSON := `{
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
		"_links": {},
		"seo": {
			"seo_title": "Custom SEO Title",
			"meta_description": "Custom meta description",
			"og_image": "https://example.com/og.jpg"
		}
	}`

	var post WordPressPost
	err := json.Unmarshal([]byte(postJSON), &post)
	if err != nil {
		t.Fatalf("Failed to unmarshal WordPressPost with SEO: %v", err)
	}

	if post.SEO.Title != "Custom SEO Title" {
		t.Errorf("WordPressPost SEO.Title = %s, want %s", post.SEO.Title, "Custom SEO Title")
	}

	if post.SEO.MetaDescription != "Custom meta description" {
		t.Errorf("WordPressPost SEO.MetaDescription = %s", post.SEO.MetaDescription)
	}

	if post.SEO.OGImage != "https://example.com/og.jpg" {
		t.Errorf("WordPressPost SEO.OGImage = %s", post.SEO.OGImage)
	}
}
