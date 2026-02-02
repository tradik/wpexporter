package checkpoint

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// State represents the current export progress
type State struct {
	mu sync.RWMutex `json:"-"`

	// Export configuration
	SiteURL   string    `json:"site_url"`
	Format    string    `json:"format"`
	OutputDir string    `json:"output_dir"`
	StartedAt time.Time `json:"started_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Progress tracking
	PostsPage           int   `json:"posts_page"`
	PostsCompleted      bool  `json:"posts_completed"`
	PostIDs             []int `json:"post_ids"`
	PagesPage           int   `json:"pages_page"`
	PagesCompleted      bool  `json:"pages_completed"`
	PageIDs             []int `json:"page_ids"`
	ProductsPage        int   `json:"products_page"`
	ProductsCompleted   bool  `json:"products_completed"`
	ProductIDs          []int `json:"product_ids"`
	MediaPage           int   `json:"media_page"`
	MediaCompleted      bool  `json:"media_completed"`
	MediaIDs            []int `json:"media_ids"`
	CategoriesCompleted bool  `json:"categories_completed"`
	TagsCompleted       bool  `json:"tags_completed"`
	UsersCompleted      bool  `json:"users_completed"`

	// Downloaded media tracking
	DownloadedMedia map[string]bool `json:"downloaded_media"`

	// Error info
	LastError string `json:"last_error,omitempty"`
}

// Manager handles checkpoint save/load operations
type Manager struct {
	filePath string
	state    *State
	enabled  bool
}

// NewManager creates a new checkpoint manager
func NewManager(outputDir string, enabled bool) *Manager {
	return &Manager{
		filePath: filepath.Join(outputDir, ".wpexport_checkpoint.json"),
		state:    nil,
		enabled:  enabled,
	}
}

// GetFilePath returns the checkpoint file path
func (m *Manager) GetFilePath() string {
	return m.filePath
}

// IsEnabled returns whether checkpointing is enabled
func (m *Manager) IsEnabled() bool {
	return m.enabled
}

// Load loads checkpoint from file if it exists
func (m *Manager) Load(siteURL string) (*State, error) {
	if !m.enabled {
		return m.newState(siteURL), nil
	}

	data, err := os.ReadFile(m.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// No checkpoint exists, create new state
			m.state = m.newState(siteURL)
			return m.state, nil
		}
		return nil, fmt.Errorf("failed to read checkpoint: %w", err)
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse checkpoint: %w", err)
	}

	// Verify checkpoint is for the same site
	if state.SiteURL != siteURL {
		return nil, fmt.Errorf("checkpoint is for different site (%s), expected %s. Delete checkpoint file to start fresh", state.SiteURL, siteURL)
	}

	// Initialize the map if nil
	if state.DownloadedMedia == nil {
		state.DownloadedMedia = make(map[string]bool)
	}

	m.state = &state
	return m.state, nil
}

// newState creates a new empty state
func (m *Manager) newState(siteURL string) *State {
	return &State{
		SiteURL:         siteURL,
		StartedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		PostsPage:       1,
		PagesPage:       1,
		ProductsPage:    1,
		MediaPage:       1,
		PostIDs:         []int{},
		PageIDs:         []int{},
		ProductIDs:      []int{},
		MediaIDs:        []int{},
		DownloadedMedia: make(map[string]bool),
	}
}

// Save saves current state to checkpoint file
func (m *Manager) Save() error {
	if !m.enabled || m.state == nil {
		return nil
	}

	m.state.mu.Lock()
	m.state.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(m.state, "", "  ")
	m.state.mu.Unlock()

	if err != nil {
		return fmt.Errorf("failed to marshal checkpoint: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(m.filePath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create checkpoint directory: %w", err)
	}

	if err := os.WriteFile(m.filePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write checkpoint: %w", err)
	}

	return nil
}

// Delete removes the checkpoint file (called on successful completion)
func (m *Manager) Delete() error {
	if !m.enabled {
		return nil
	}

	err := os.Remove(m.filePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete checkpoint: %w", err)
	}
	return nil
}

// Exists checks if a checkpoint file exists
func (m *Manager) Exists() bool {
	_, err := os.Stat(m.filePath)
	return err == nil
}

// GetState returns the current state
func (m *Manager) GetState() *State {
	return m.state
}

// SetState sets the current state
func (m *Manager) SetState(state *State) {
	m.state = state
}

// State update methods (thread-safe)

// SetPostsPage updates the posts pagination progress
func (s *State) SetPostsPage(page int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.PostsPage = page
}

// AddPostIDs adds processed post IDs
func (s *State) AddPostIDs(ids []int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.PostIDs = append(s.PostIDs, ids...)
}

// SetPostsCompleted marks posts as fully fetched
func (s *State) SetPostsCompleted() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.PostsCompleted = true
}

// SetPagesPage updates the pages pagination progress
func (s *State) SetPagesPage(page int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.PagesPage = page
}

// AddPageIDs adds processed page IDs
func (s *State) AddPageIDs(ids []int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.PageIDs = append(s.PageIDs, ids...)
}

// SetPagesCompleted marks pages as fully fetched
func (s *State) SetPagesCompleted() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.PagesCompleted = true
}

// SetProductsPage updates the products pagination progress
func (s *State) SetProductsPage(page int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ProductsPage = page
}

// AddProductIDs adds processed product IDs
func (s *State) AddProductIDs(ids []int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ProductIDs = append(s.ProductIDs, ids...)
}

// SetProductsCompleted marks products as fully fetched
func (s *State) SetProductsCompleted() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ProductsCompleted = true
}

// SetMediaPage updates the media pagination progress
func (s *State) SetMediaPage(page int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.MediaPage = page
}

// AddMediaIDs adds processed media IDs
func (s *State) AddMediaIDs(ids []int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.MediaIDs = append(s.MediaIDs, ids...)
}

// SetMediaCompleted marks media as fully fetched
func (s *State) SetMediaCompleted() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.MediaCompleted = true
}

// SetCategoriesCompleted marks categories as fully fetched
func (s *State) SetCategoriesCompleted() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CategoriesCompleted = true
}

// SetTagsCompleted marks tags as fully fetched
func (s *State) SetTagsCompleted() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TagsCompleted = true
}

// SetUsersCompleted marks users as fully fetched
func (s *State) SetUsersCompleted() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.UsersCompleted = true
}

// MarkMediaDownloaded marks a media URL as downloaded
func (s *State) MarkMediaDownloaded(url string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.DownloadedMedia[url] = true
}

// IsMediaDownloaded checks if a media URL was already downloaded
func (s *State) IsMediaDownloaded(url string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.DownloadedMedia[url]
}

// SetLastError records the last error
func (s *State) SetLastError(err string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastError = err
}

// GetPostsPage returns the current posts page
func (s *State) GetPostsPage() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.PostsPage
}

// GetPagesPage returns the current pages page
func (s *State) GetPagesPage() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.PagesPage
}

// GetProductsPage returns the current products page
func (s *State) GetProductsPage() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ProductsPage
}

// GetMediaPage returns the current media page
func (s *State) GetMediaPage() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.MediaPage
}

// IsPostsCompleted returns whether posts fetching is complete
func (s *State) IsPostsCompleted() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.PostsCompleted
}

// IsPagesCompleted returns whether pages fetching is complete
func (s *State) IsPagesCompleted() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.PagesCompleted
}

// IsProductsCompleted returns whether products fetching is complete
func (s *State) IsProductsCompleted() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ProductsCompleted
}

// IsMediaCompleted returns whether media fetching is complete
func (s *State) IsMediaCompleted() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.MediaCompleted
}

// IsCategoriesCompleted returns whether categories fetching is complete
func (s *State) IsCategoriesCompleted() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.CategoriesCompleted
}

// IsTagsCompleted returns whether tags fetching is complete
func (s *State) IsTagsCompleted() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.TagsCompleted
}

// IsUsersCompleted returns whether users fetching is complete
func (s *State) IsUsersCompleted() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.UsersCompleted
}

// Summary returns a human-readable summary of checkpoint state
func (s *State) Summary() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return fmt.Sprintf(
		"Checkpoint: posts=%d (done=%v), pages=%d (done=%v), products=%d (done=%v), media=%d (done=%v), downloaded=%d",
		len(s.PostIDs), s.PostsCompleted,
		len(s.PageIDs), s.PagesCompleted,
		len(s.ProductIDs), s.ProductsCompleted,
		len(s.MediaIDs), s.MediaCompleted,
		len(s.DownloadedMedia),
	)
}
