package bruteforce

import (
	"fmt"
	"sync"
	"time"

	"github.com/schollz/progressbar/v3"
	"github.com/tradik/wpexportjson/internal/api"
	"github.com/tradik/wpexportjson/internal/config"
	"github.com/tradik/wpexportjson/pkg/models"
)

// Scanner handles brute force content discovery
type Scanner struct {
	config    *config.Config
	apiClient *api.Client
}

// NewScanner creates a new brute force scanner
func NewScanner(cfg *config.Config, client *api.Client) *Scanner {
	return &Scanner{
		config:    cfg,
		apiClient: client,
	}
}

// ScanResult represents the result of a brute force scan
type ScanResult struct {
	Posts []models.WordPressPost
	Pages []models.WordPressPost
	Media []models.WordPressMedia
	Found int
}

// ScanForContent performs brute force scanning for missing content
func (s *Scanner) ScanForContent(existingPosts, existingPages []models.WordPressPost, existingMedia []models.WordPressMedia) (*ScanResult, error) {
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
	var wg sync.WaitGroup

	// Scan for posts
	wg.Add(1)
	go func() {
		defer wg.Done()
		posts := s.scanPosts(existingPostIDs)
		result.Posts = posts
		result.Found += len(posts)
	}()

	// Scan for pages
	wg.Add(1)
	go func() {
		defer wg.Done()
		pages := s.scanPages(existingPageIDs)
		result.Pages = pages
		result.Found += len(pages)
	}()

	// Scan for media
	wg.Add(1)
	go func() {
		defer wg.Done()
		media := s.scanMedia(existingMediaIDs)
		result.Media = media
		result.Found += len(media)
	}()

	wg.Wait()

	if result.Found > 0 {
		fmt.Printf("Brute force scan found %d additional items\n", result.Found)
	} else {
		fmt.Println("Brute force scan completed - no additional content found")
	}

	return result, nil
}

// scanPosts scans for posts using brute force
func (s *Scanner) scanPosts(existingIDs map[int]bool) []models.WordPressPost {
	fmt.Println("Scanning for missing posts...")
	
	progress := progressbar.NewOptions(s.config.MaxID,
		progressbar.OptionSetDescription("Scanning posts"),
		progressbar.OptionSetWidth(50),
		progressbar.OptionShowCount(),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "=",
			SaucerHead:    ">",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
	)

	var foundPosts []models.WordPressPost
	var mutex sync.Mutex

	// Create worker pool
	jobs := make(chan int, s.config.MaxID)
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < s.config.Concurrent; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range jobs {
				if !existingIDs[id] {
					post, err := s.apiClient.GetPostByID(id)
					if err == nil && post != nil {
						mutex.Lock()
						foundPosts = append(foundPosts, *post)
						mutex.Unlock()
						
						if s.config.Verbose {
							fmt.Printf("Found post: ID %d - %s\n", post.ID, post.Title.Rendered)
						}
					}
				}
				progress.Add(1)
				
				// Small delay to avoid overwhelming the server
				time.Sleep(10 * time.Millisecond)
			}
		}()
	}

	// Send jobs
	for id := 1; id <= s.config.MaxID; id++ {
		jobs <- id
	}
	close(jobs)

	wg.Wait()
	progress.Finish()

	return foundPosts
}

// scanPages scans for pages using brute force
func (s *Scanner) scanPages(existingIDs map[int]bool) []models.WordPressPost {
	fmt.Println("Scanning for missing pages...")
	
	progress := progressbar.NewOptions(s.config.MaxID,
		progressbar.OptionSetDescription("Scanning pages"),
		progressbar.OptionSetWidth(50),
		progressbar.OptionShowCount(),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "=",
			SaucerHead:    ">",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
	)

	var foundPages []models.WordPressPost
	var mutex sync.Mutex

	// Create worker pool
	jobs := make(chan int, s.config.MaxID)
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < s.config.Concurrent; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range jobs {
				if !existingIDs[id] {
					page, err := s.apiClient.GetPageByID(id)
					if err == nil && page != nil {
						mutex.Lock()
						foundPages = append(foundPages, *page)
						mutex.Unlock()
						
						if s.config.Verbose {
							fmt.Printf("Found page: ID %d - %s\n", page.ID, page.Title.Rendered)
						}
					}
				}
				progress.Add(1)
				
				// Small delay to avoid overwhelming the server
				time.Sleep(10 * time.Millisecond)
			}
		}()
	}

	// Send jobs
	for id := 1; id <= s.config.MaxID; id++ {
		jobs <- id
	}
	close(jobs)

	wg.Wait()
	progress.Finish()

	return foundPages
}

// scanMedia scans for media using brute force
func (s *Scanner) scanMedia(existingIDs map[int]bool) []models.WordPressMedia {
	fmt.Println("Scanning for missing media...")
	
	progress := progressbar.NewOptions(s.config.MaxID,
		progressbar.OptionSetDescription("Scanning media"),
		progressbar.OptionSetWidth(50),
		progressbar.OptionShowCount(),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "=",
			SaucerHead:    ">",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
	)

	var foundMedia []models.WordPressMedia
	var mutex sync.Mutex

	// Create worker pool
	jobs := make(chan int, s.config.MaxID)
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < s.config.Concurrent; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range jobs {
				if !existingIDs[id] {
					media, err := s.apiClient.GetMediaByID(id)
					if err == nil && media != nil {
						mutex.Lock()
						foundMedia = append(foundMedia, *media)
						mutex.Unlock()
						
						if s.config.Verbose {
							fmt.Printf("Found media: ID %d - %s\n", media.ID, media.Title.Rendered)
						}
					}
				}
				progress.Add(1)
				
				// Small delay to avoid overwhelming the server
				time.Sleep(10 * time.Millisecond)
			}
		}()
	}

	// Send jobs
	for id := 1; id <= s.config.MaxID; id++ {
		jobs <- id
	}
	close(jobs)

	wg.Wait()
	progress.Finish()

	return foundMedia
}

// ScanSpecificRange scans a specific range of IDs for a content type
func (s *Scanner) ScanSpecificRange(contentType string, startID, endID int) (interface{}, error) {
	fmt.Printf("Scanning %s IDs from %d to %d...\n", contentType, startID, endID)
	
	total := endID - startID + 1
	progress := progressbar.NewOptions(total,
		progressbar.OptionSetDescription(fmt.Sprintf("Scanning %s", contentType)),
		progressbar.OptionSetWidth(50),
		progressbar.OptionShowCount(),
	)

	switch contentType {
	case "posts":
		var posts []models.WordPressPost
		for id := startID; id <= endID; id++ {
			post, err := s.apiClient.GetPostByID(id)
			if err == nil && post != nil {
				posts = append(posts, *post)
			}
			progress.Add(1)
			time.Sleep(10 * time.Millisecond)
		}
		progress.Finish()
		return posts, nil
		
	case "pages":
		var pages []models.WordPressPost
		for id := startID; id <= endID; id++ {
			page, err := s.apiClient.GetPageByID(id)
			if err == nil && page != nil {
				pages = append(pages, *page)
			}
			progress.Add(1)
			time.Sleep(10 * time.Millisecond)
		}
		progress.Finish()
		return pages, nil
		
	case "media":
		var media []models.WordPressMedia
		for id := startID; id <= endID; id++ {
			mediaItem, err := s.apiClient.GetMediaByID(id)
			if err == nil && mediaItem != nil {
				media = append(media, *mediaItem)
			}
			progress.Add(1)
			time.Sleep(10 * time.Millisecond)
		}
		progress.Finish()
		return media, nil
		
	default:
		return nil, fmt.Errorf("unsupported content type: %s", contentType)
	}
}
