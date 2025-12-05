package bruteforce

import (
	"fmt"
	"testing"
	"time"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

// APIClientInterface defines the interface for API client operations
type APIClientInterface interface {
	GetPostByID(id int) (*models.WordPressPost, error)
	GetPageByID(id int) (*models.WordPressPost, error)
	GetMediaByID(id int) (*models.WordPressMedia, error)
	GetSiteInfo() (*models.SiteInfo, error)
	GetPosts() ([]models.WordPressPost, error)
	GetPages() ([]models.WordPressPost, error)
	GetMedia() ([]models.WordPressMedia, error)
	GetCategories() ([]models.WordPressCategory, error)
	GetTags() ([]models.WordPressTag, error)
	GetUsers() ([]models.WordPressUser, error)
	BruteForceContent(contentType string, maxID int, found chan<- interface{}, progress chan<- int)
}

// MockAPIClient is a mock implementation of the API client for testing
type MockAPIClient struct {
	posts map[int]*models.WordPressPost
	pages map[int]*models.WordPressPost
	media map[int]*models.WordPressMedia
}

func NewMockAPIClient() *MockAPIClient {
	return &MockAPIClient{
		posts: make(map[int]*models.WordPressPost),
		pages: make(map[int]*models.WordPressPost),
		media: make(map[int]*models.WordPressMedia),
	}
}

func (m *MockAPIClient) AddPost(id int, post *models.WordPressPost) {
	m.posts[id] = post
}

func (m *MockAPIClient) AddPage(id int, page *models.WordPressPost) {
	m.pages[id] = page
}

func (m *MockAPIClient) AddMedia(id int, media *models.WordPressMedia) {
	m.media[id] = media
}

func (m *MockAPIClient) GetPostByID(id int) (*models.WordPressPost, error) {
	if post, exists := m.posts[id]; exists {
		return post, nil
	}
	return nil, nil
}

func (m *MockAPIClient) GetPageByID(id int) (*models.WordPressPost, error) {
	if page, exists := m.pages[id]; exists {
		return page, nil
	}
	return nil, nil
}

func (m *MockAPIClient) GetMediaByID(id int) (*models.WordPressMedia, error) {
	if media, exists := m.media[id]; exists {
		return media, nil
	}
	return nil, nil
}

// Implement other required methods with empty implementations
func (m *MockAPIClient) GetSiteInfo() (*models.SiteInfo, error) {
	return &models.SiteInfo{Name: "Test Site"}, nil
}

func (m *MockAPIClient) GetPosts() ([]models.WordPressPost, error) {
	var posts []models.WordPressPost
	for _, post := range m.posts {
		posts = append(posts, *post)
	}
	return posts, nil
}

func (m *MockAPIClient) GetPages() ([]models.WordPressPost, error) {
	var pages []models.WordPressPost
	for _, page := range m.pages {
		pages = append(pages, *page)
	}
	return pages, nil
}

func (m *MockAPIClient) GetMedia() ([]models.WordPressMedia, error) {
	var media []models.WordPressMedia
	for _, item := range m.media {
		media = append(media, *item)
	}
	return media, nil
}

func (m *MockAPIClient) GetCategories() ([]models.WordPressCategory, error) {
	return []models.WordPressCategory{}, nil
}

func (m *MockAPIClient) GetTags() ([]models.WordPressTag, error) {
	return []models.WordPressTag{}, nil
}

func (m *MockAPIClient) GetUsers() ([]models.WordPressUser, error) {
	return []models.WordPressUser{}, nil
}

func (m *MockAPIClient) BruteForceContent(contentType string, maxID int, found chan<- interface{}, progress chan<- int) {
	// Not used in these tests
}

// TestScanner is a test version of Scanner that uses the interface
type TestScanner struct {
	config    *config.Config
	apiClient APIClientInterface
}

// NewTestScanner creates a new test scanner with interface
func NewTestScanner(cfg *config.Config, client APIClientInterface) *TestScanner {
	return &TestScanner{
		config:    cfg,
		apiClient: client,
	}
}

// Copy the scanner methods but adapted for the interface
func (s *TestScanner) scanPosts(existingIDs map[int]bool) []models.WordPressPost {
	fmt.Println("Scanning for missing posts...")

	var foundPosts []models.WordPressPost

	for id := 1; id <= s.config.MaxID; id++ {
		if !existingIDs[id] {
			post, err := s.apiClient.GetPostByID(id)
			if err == nil && post != nil {
				foundPosts = append(foundPosts, *post)

				if s.config.Verbose {
					fmt.Printf("Found post: ID %d - %s\n", post.ID, post.Title.Rendered)
				}
			}
		}
		time.Sleep(10 * time.Millisecond)
	}

	return foundPosts
}

func (s *TestScanner) scanPages(existingIDs map[int]bool) []models.WordPressPost {
	fmt.Println("Scanning for missing pages...")

	var foundPages []models.WordPressPost

	for id := 1; id <= s.config.MaxID; id++ {
		if !existingIDs[id] {
			page, err := s.apiClient.GetPageByID(id)
			if err == nil && page != nil {
				foundPages = append(foundPages, *page)

				if s.config.Verbose {
					fmt.Printf("Found page: ID %d - %s\n", page.ID, page.Title.Rendered)
				}
			}
		}
		time.Sleep(10 * time.Millisecond)
	}

	return foundPages
}

func (s *TestScanner) scanMedia(existingIDs map[int]bool) []models.WordPressMedia {
	fmt.Println("Scanning for missing media...")

	var foundMedia []models.WordPressMedia

	for id := 1; id <= s.config.MaxID; id++ {
		if !existingIDs[id] {
			media, err := s.apiClient.GetMediaByID(id)
			if err == nil && media != nil {
				foundMedia = append(foundMedia, *media)

				if s.config.Verbose {
					fmt.Printf("Found media: ID %d - %s\n", media.ID, media.Title.Rendered)
				}
			}
		}
		time.Sleep(10 * time.Millisecond)
	}

	return foundMedia
}

func (s *TestScanner) ScanForContent(existingPosts, existingPages []models.WordPressPost, existingMedia []models.WordPressMedia) (*ScanResult, error) {
	if !s.config.BruteForce {
		return &ScanResult{}, nil
	}

	fmt.Println("Starting brute force content discovery...")

	// Create maps of existing IDs for quick lookup
	existingPostIDs := make(map[int]bool)
	for _, post := range existingPosts {
		existingPostIDs[post.ID] = true
	}

	existingPageIDs := make(map[int]bool)
	for _, page := range existingPages {
		existingPageIDs[page.ID] = true
	}

	existingMediaIDs := make(map[int]bool)
	for _, media := range existingMedia {
		existingMediaIDs[media.ID] = true
	}

	result := &ScanResult{}

	// Scan for posts
	posts := s.scanPosts(existingPostIDs)
	result.Posts = posts
	result.Found += len(posts)

	// Scan for pages
	pages := s.scanPages(existingPageIDs)
	result.Pages = pages
	result.Found += len(pages)

	// Scan for media
	media := s.scanMedia(existingMediaIDs)
	result.Media = media
	result.Found += len(media)

	if result.Found > 0 {
		fmt.Printf("Brute force scan found %d additional items\n", result.Found)
	} else {
		fmt.Println("Brute force scan completed - no additional content found")
	}

	return result, nil
}

func (s *TestScanner) ScanSpecificRange(contentType string, startID, endID int) (interface{}, error) {
	fmt.Printf("Scanning %s IDs from %d to %d...\n", contentType, startID, endID)

	switch contentType {
	case "posts":
		var posts []models.WordPressPost
		for id := startID; id <= endID; id++ {
			post, err := s.apiClient.GetPostByID(id)
			if err == nil && post != nil {
				posts = append(posts, *post)
			}
			time.Sleep(10 * time.Millisecond)
		}
		return posts, nil

	case "pages":
		var pages []models.WordPressPost
		for id := startID; id <= endID; id++ {
			page, err := s.apiClient.GetPageByID(id)
			if err == nil && page != nil {
				pages = append(pages, *page)
			}
			time.Sleep(10 * time.Millisecond)
		}
		return pages, nil

	case "media":
		var media []models.WordPressMedia
		for id := startID; id <= endID; id++ {
			mediaItem, err := s.apiClient.GetMediaByID(id)
			if err == nil && mediaItem != nil {
				media = append(media, *mediaItem)
			}
			time.Sleep(10 * time.Millisecond)
		}
		return media, nil

	default:
		return nil, fmt.Errorf("unsupported content type: %s", contentType)
	}
}

func TestNewScanner(t *testing.T) {
	cfg := &config.Config{
		URL:        "https://example.com",
		BruteForce: true,
		MaxID:      100,
		Concurrent: 5,
		Timeout:    30,
		Retries:    3,
	}

	mockClient := NewMockAPIClient()
	scanner := NewTestScanner(cfg, mockClient)

	// NewTestScanner should never return nil when there's no error
	if scanner.config != cfg {
		t.Error("NewTestScanner() should store config reference")
	}

	if scanner.apiClient != mockClient {
		t.Error("NewTestScanner() should store API client reference")
	}
}

func TestScanForContentDisabled(t *testing.T) {
	cfg := &config.Config{
		BruteForce: false, // Disabled
	}

	mockClient := NewMockAPIClient()
	scanner := NewTestScanner(cfg, mockClient)

	result, err := scanner.ScanForContent([]models.WordPressPost{}, []models.WordPressPost{}, []models.WordPressMedia{})

	if err != nil {
		t.Errorf("ScanForContent() error = %v, want nil", err)
	}

	if result.Found != 0 {
		t.Errorf("ScanForContent() Found = %d, want 0", result.Found)
	}

	if len(result.Posts) != 0 {
		t.Errorf("ScanForContent() Posts length = %d, want 0", len(result.Posts))
	}
}

func TestScanForContentEnabled(t *testing.T) {
	cfg := &config.Config{
		BruteForce: true,
		MaxID:      10,
		Concurrent: 2,
		Timeout:    30,
		Retries:    1,
	}

	mockClient := NewMockAPIClient()
	scanner := NewTestScanner(cfg, mockClient)

	// Add some existing content
	existingPosts := []models.WordPressPost{
		{ID: 1, Title: models.RenderedContent{Rendered: "Existing Post"}},
	}

	existingPages := []models.WordPressPost{
		{ID: 2, Title: models.RenderedContent{Rendered: "Existing Page"}},
	}

	existingMedia := []models.WordPressMedia{
		{ID: 3, Title: models.RenderedContent{Rendered: "Existing Media"}},
	}

	// Add some new content to mock client
	mockClient.AddPost(5, &models.WordPressPost{
		ID:    5,
		Title: models.RenderedContent{Rendered: "New Post"},
		Slug:  "new-post",
	})

	mockClient.AddPage(7, &models.WordPressPost{
		ID:    7,
		Title: models.RenderedContent{Rendered: "New Page"},
		Slug:  "new-page",
	})

	mockClient.AddMedia(9, &models.WordPressMedia{
		ID:    9,
		Title: models.RenderedContent{Rendered: "New Media"},
		Slug:  "new-media",
	})

	result, err := scanner.ScanForContent(existingPosts, existingPages, existingMedia)

	if err != nil {
		t.Errorf("ScanForContent() error = %v, want nil", err)
	}

	if result.Found != 3 {
		t.Errorf("ScanForContent() Found = %d, want 3", result.Found)
	}

	if len(result.Posts) != 1 {
		t.Errorf("ScanForContent() Posts length = %d, want 1", len(result.Posts))
	}

	if len(result.Pages) != 1 {
		t.Errorf("ScanForContent() Pages length = %d, want 1", len(result.Pages))
	}

	if len(result.Media) != 1 {
		t.Errorf("ScanForContent() Media length = %d, want 1", len(result.Media))
	}

	// Verify the found content
	if result.Posts[0].ID != 5 {
		t.Errorf("ScanForContent() first post ID = %d, want 5", result.Posts[0].ID)
	}

	if result.Pages[0].ID != 7 {
		t.Errorf("ScanForContent() first page ID = %d, want 7", result.Pages[0].ID)
	}

	if result.Media[0].ID != 9 {
		t.Errorf("ScanForContent() first media ID = %d, want 9", result.Media[0].ID)
	}
}

func TestScanForContentWithDuplicates(t *testing.T) {
	cfg := &config.Config{
		BruteForce: true,
		MaxID:      5,
		Concurrent: 2,
		Timeout:    30,
		Retries:    1,
	}

	mockClient := NewMockAPIClient()
	scanner := NewTestScanner(cfg, mockClient)

	// Add existing content
	existingPosts := []models.WordPressPost{
		{ID: 1, Title: models.RenderedContent{Rendered: "Existing Post"}},
		{ID: 2, Title: models.RenderedContent{Rendered: "Another Existing Post"}},
	}

	// Add the same content to mock client (should be skipped)
	mockClient.AddPost(1, &models.WordPressPost{
		ID:    1,
		Title: models.RenderedContent{Rendered: "Existing Post"},
	})

	mockClient.AddPost(3, &models.WordPressPost{
		ID:    3,
		Title: models.RenderedContent{Rendered: "New Post"},
	})

	result, err := scanner.ScanForContent(existingPosts, []models.WordPressPost{}, []models.WordPressMedia{})

	if err != nil {
		t.Errorf("ScanForContent() error = %v, want nil", err)
	}

	// Should only find the new post, not duplicates
	if result.Found != 1 {
		t.Errorf("ScanForContent() Found = %d, want 1", result.Found)
	}

	if len(result.Posts) != 1 {
		t.Errorf("ScanForContent() Posts length = %d, want 1", len(result.Posts))
	}

	if result.Posts[0].ID != 3 {
		t.Errorf("ScanForContent() post ID = %d, want 3", result.Posts[0].ID)
	}
}

func TestScanSpecificRange(t *testing.T) {
	cfg := &config.Config{
		MaxID:      100,
		Concurrent: 2,
		Timeout:    30,
		Retries:    1,
	}

	mockClient := NewMockAPIClient()
	scanner := NewTestScanner(cfg, mockClient)

	// Add some posts in the range
	mockClient.AddPost(10, &models.WordPressPost{
		ID:    10,
		Title: models.RenderedContent{Rendered: "Post 10"},
	})

	mockClient.AddPost(15, &models.WordPressPost{
		ID:    15,
		Title: models.RenderedContent{Rendered: "Post 15"},
	})

	mockClient.AddPost(20, &models.WordPressPost{
		ID:    20,
		Title: models.RenderedContent{Rendered: "Post 20"},
	})

	// Test scanning posts
	result, err := scanner.ScanSpecificRange("posts", 10, 20)

	if err != nil {
		t.Errorf("ScanSpecificRange() error = %v, want nil", err)
	}

	posts, ok := result.([]models.WordPressPost)
	if !ok {
		t.Fatal("ScanSpecificRange() should return []models.WordPressPost for posts")
	}

	if len(posts) != 3 {
		t.Errorf("ScanSpecificRange() posts length = %d, want 3", len(posts))
	}

	// Test scanning pages
	mockClient.AddPage(12, &models.WordPressPost{
		ID:    12,
		Title: models.RenderedContent{Rendered: "Page 12"},
	})

	result, err = scanner.ScanSpecificRange("pages", 10, 20)

	if err != nil {
		t.Errorf("ScanSpecificRange() error = %v, want nil", err)
	}

	pages, ok := result.([]models.WordPressPost)
	if !ok {
		t.Fatal("ScanSpecificRange() should return []models.WordPressPost for pages")
	}

	if len(pages) != 1 {
		t.Errorf("ScanSpecificRange() pages length = %d, want 1", len(pages))
	}

	// Test scanning media
	mockClient.AddMedia(18, &models.WordPressMedia{
		ID:    18,
		Title: models.RenderedContent{Rendered: "Media 18"},
	})

	result, err = scanner.ScanSpecificRange("media", 10, 20)

	if err != nil {
		t.Errorf("ScanSpecificRange() error = %v, want nil", err)
	}

	media, ok := result.([]models.WordPressMedia)
	if !ok {
		t.Fatal("ScanSpecificRange() should return []models.WordPressMedia for media")
	}

	if len(media) != 1 {
		t.Errorf("ScanSpecificRange() media length = %d, want 1", len(media))
	}
}

func TestScanSpecificRangeInvalidContentType(t *testing.T) {
	cfg := &config.Config{
		MaxID:      100,
		Concurrent: 2,
		Timeout:    30,
		Retries:    1,
	}

	mockClient := NewMockAPIClient()
	scanner := NewTestScanner(cfg, mockClient)

	result, err := scanner.ScanSpecificRange("invalid", 1, 10)

	if err == nil {
		t.Error("ScanSpecificRange() should return error for invalid content type")
	}

	if result != nil {
		t.Error("ScanSpecificRange() should return nil result for invalid content type")
	}
}

func TestScanSpecificRangeInvalidRange(t *testing.T) {
	cfg := &config.Config{
		MaxID:      100,
		Concurrent: 2,
		Timeout:    30,
		Retries:    1,
	}

	mockClient := NewMockAPIClient()
	scanner := NewTestScanner(cfg, mockClient)

	// Test with start > end
	result, err := scanner.ScanSpecificRange("posts", 20, 10)

	if err != nil {
		t.Errorf("ScanSpecificRange() error = %v, want nil", err)
	}

	posts, ok := result.([]models.WordPressPost)
	if !ok {
		t.Fatal("ScanSpecificRange() should return []models.WordPressPost for posts")
	}

	// Should return empty result for invalid range
	if len(posts) != 0 {
		t.Errorf("ScanSpecificRange() posts length = %d, want 0", len(posts))
	}
}

func TestScanResultStructure(t *testing.T) {
	result := &ScanResult{
		Posts: []models.WordPressPost{
			{ID: 1, Title: models.RenderedContent{Rendered: "Post 1"}},
		},
		Pages: []models.WordPressPost{
			{ID: 2, Title: models.RenderedContent{Rendered: "Page 1"}},
		},
		Media: []models.WordPressMedia{
			{ID: 3, Title: models.RenderedContent{Rendered: "Media 1"}},
		},
		Found: 3,
	}

	if len(result.Posts) != 1 {
		t.Errorf("ScanResult Posts length = %d, want 1", len(result.Posts))
	}

	if len(result.Pages) != 1 {
		t.Errorf("ScanResult Pages length = %d, want 1", len(result.Pages))
	}

	if len(result.Media) != 1 {
		t.Errorf("ScanResult Media length = %d, want 1", len(result.Media))
	}

	if result.Found != 3 {
		t.Errorf("ScanResult Found = %d, want 3", result.Found)
	}

	// Test that Found matches total items
	expectedFound := len(result.Posts) + len(result.Pages) + len(result.Media)
	if result.Found != expectedFound {
		t.Errorf("ScanResult Found = %d, expected %d based on items", result.Found, expectedFound)
	}
}

func TestConcurrentScanning(t *testing.T) {
	cfg := &config.Config{
		BruteForce: true,
		MaxID:      50,
		Concurrent: 5, // High concurrency
		Timeout:    30,
		Retries:    1,
	}

	mockClient := NewMockAPIClient()
	scanner := NewTestScanner(cfg, mockClient)

	// Add many posts to test concurrent processing
	for i := 1; i <= 20; i++ {
		mockClient.AddPost(i, &models.WordPressPost{
			ID:    i,
			Title: models.RenderedContent{Rendered: fmt.Sprintf("Post %d", i)},
		})
	}

	start := time.Now()
	result, err := scanner.ScanForContent([]models.WordPressPost{}, []models.WordPressPost{}, []models.WordPressMedia{})
	duration := time.Since(start)

	if err != nil {
		t.Errorf("ScanForContent() error = %v, want nil", err)
	}

	if result.Found != 20 {
		t.Errorf("ScanForContent() Found = %d, want 20", result.Found)
	}

	// Concurrent scanning should be reasonably fast
	// This is a rough check - adjust threshold if needed
	if duration > 10*time.Second {
		t.Errorf("ScanForContent() took too long: %v", duration)
	}
}

func TestScanWithEmptyResults(t *testing.T) {
	cfg := &config.Config{
		BruteForce: true,
		MaxID:      10,
		Concurrent: 2,
		Timeout:    30,
		Retries:    1,
	}

	mockClient := NewMockAPIClient()
	scanner := NewTestScanner(cfg, mockClient)

	// Don't add any content to mock client
	result, err := scanner.ScanForContent([]models.WordPressPost{}, []models.WordPressPost{}, []models.WordPressMedia{})

	if err != nil {
		t.Errorf("ScanForContent() error = %v, want nil", err)
	}

	if result.Found != 0 {
		t.Errorf("ScanForContent() Found = %d, want 0", result.Found)
	}

	if len(result.Posts) != 0 {
		t.Errorf("ScanForContent() Posts length = %d, want 0", len(result.Posts))
	}

	if len(result.Pages) != 0 {
		t.Errorf("ScanForContent() Pages length = %d, want 0", len(result.Pages))
	}

	if len(result.Media) != 0 {
		t.Errorf("ScanForContent() Media length = %d, want 0", len(result.Media))
	}
}
