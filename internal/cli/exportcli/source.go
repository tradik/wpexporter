package exportcli

import (
	"github.com/tradik/wpexporter/internal/api"
	"github.com/tradik/wpexporter/internal/bruteforce"
	"github.com/tradik/wpexporter/pkg/models"
)

// contentSource is what the export reads a site from. *api.Client satisfies it
// over REST; *localdb.Source satisfies it from the database (FEAT-001). The
// interface lives here, with its consumer, and holds only what the shared
// stages call.
type contentSource interface {
	GetSiteInfo() (*models.SiteInfo, error)
	GetPosts() ([]models.WordPressPost, error)
	GetPages() ([]models.WordPressPost, error)
	GetMedia() ([]models.WordPressMedia, error)
	GetCategories() ([]models.WordPressCategory, error)
	GetTags() ([]models.WordPressTag, error)
	GetUsers() ([]models.WordPressUser, error)
	menuSource
	commentSource
	customTypeSource
}

// commentSource is what fetchComments reads from.
type commentSource interface {
	GetComments() ([]models.WordPressComment, error)
}

// customTypeSource is what fetchCustomTypes reads from: the types a site
// registers, and one type's entries.
type customTypeSource interface {
	GetPostTypes() ([]api.PostType, error)
	GetCustomPosts(restBase string) ([]models.WordPressPost, error)
}

// contentScanner discovers content by ID: brute force and the range rescan.
// *bruteforce.Scanner is one.
type contentScanner interface {
	ScanForContent(posts, pages []models.WordPressPost, media []models.WordPressMedia) (*bruteforce.ScanResult, error)
	ScanSpecificRange(contentType string, startID, endID int) (interface{}, error)
}

// The REST client is a content source.
var _ contentSource = (*api.Client)(nil)

// The brute force scanner is a content scanner.
var _ contentScanner = (*bruteforce.Scanner)(nil)
