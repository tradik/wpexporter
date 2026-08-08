package models

import (
	"encoding/json"
	"time"
)

// WordPressTime is a custom time type that can handle WordPress date formats
type WordPressTime struct {
	time.Time
}

// UnmarshalJSON implements json.Unmarshaler for WordPressTime
func (wt *WordPressTime) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	// Try different WordPress date formats
	formats := []string{
		"2006-01-02T15:04:05",       // WordPress format without timezone
		"2006-01-02T15:04:05Z",      // ISO format with Z
		"2006-01-02T15:04:05-07:00", // ISO format with timezone offset
		"2006-01-02T15:04:05+00:00", // ISO format with UTC offset
		time.RFC3339,                // Standard RFC3339
		time.RFC3339Nano,            // RFC3339 with nanoseconds
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			wt.Time = t
			return nil
		}
	}

	// Default to current time if parsing fails
	wt.Time = time.Now()
	return nil
}

// HreflangLink represents a single hreflang alternate link
type HreflangLink struct {
	Lang string `json:"lang"`
	Href string `json:"href"`
}

// SEOData holds extracted SEO metadata from page crawling.
//
// The named fields are the ones a static site generator reads directly. Meta
// carries everything else the page declared: plugins and themes put real
// information in tags nobody anticipated, and an export that drops them loses
// it silently.
type SEOData struct {
	Title            string         `json:"seo_title,omitempty"`
	MetaDescription  string         `json:"meta_description,omitempty"`
	MetaKeywords     string         `json:"meta_keywords,omitempty"`
	Robots           string         `json:"robots,omitempty"`
	OGTitle          string         `json:"og_title,omitempty"`
	OGDescription    string         `json:"og_description,omitempty"`
	OGImage          string         `json:"og_image,omitempty"`
	OGType           string         `json:"og_type,omitempty"`
	OGURL            string         `json:"og_url,omitempty"`
	OGSiteName       string         `json:"og_site_name,omitempty"`
	OGLocale         string         `json:"og_locale,omitempty"`
	TwitterCard      string         `json:"twitter_card,omitempty"`
	TwitterTitle     string         `json:"twitter_title,omitempty"`
	TwitterDesc      string         `json:"twitter_description,omitempty"`
	TwitterImage     string         `json:"twitter_image,omitempty"`
	TwitterSite      string         `json:"twitter_site,omitempty"`
	ArticlePublished string         `json:"article_published_time,omitempty"`
	ArticleModified  string         `json:"article_modified_time,omitempty"`
	ArticleAuthor    string         `json:"article_author,omitempty"`
	ArticleSection   string         `json:"article_section,omitempty"`
	CanonicalURL     string         `json:"canonical_url,omitempty"`
	Lang             string         `json:"lang,omitempty"`
	Hreflangs        []HreflangLink `json:"hreflangs,omitempty"`
	// Meta holds every other meta tag the page declared, keyed by its name or
	// property attribute.
	Meta map[string]string `json:"meta,omitempty"`
	// JSONLD holds the raw application/ld+json blocks. Rank Math and Yoast emit
	// structured data there that appears in no meta tag.
	JSONLD []string `json:"json_ld,omitempty"`
}

// Analytics holds tracking identifiers found in a site's pages. They are a
// property of the site rather than of any one post, and an operator rebuilding
// elsewhere needs them to carry the same measurement over.
type Analytics struct {
	GA4                 []string `json:"ga4,omitempty"`                   // G-XXXXXXXXXX
	UniversalAnalytics  []string `json:"universal_analytics,omitempty"`   // UA-XXXXXX-Y
	GoogleTagManager    []string `json:"google_tag_manager,omitempty"`    // GTM-XXXXXXX
	GoogleAdsConversion []string `json:"google_ads_conversion,omitempty"` // AW-XXXXXXXXX
	MetaPixel           []string `json:"meta_pixel,omitempty"`
	HotjarSiteID        []string `json:"hotjar_site_id,omitempty"`
	ClarityProjectID    []string `json:"clarity_project_id,omitempty"`
	LinkedInPartnerID   []string `json:"linkedin_partner_id,omitempty"`
	TikTokPixel         []string `json:"tiktok_pixel,omitempty"`
}

// WordPressPost represents a WordPress post or page
type WordPressPost struct {
	ID            int                    `json:"id"`
	Date          WordPressTime          `json:"date"`
	DateGMT       WordPressTime          `json:"date_gmt"`
	GUID          GUID                   `json:"guid"`
	Modified      WordPressTime          `json:"modified"`
	ModifiedGMT   WordPressTime          `json:"modified_gmt"`
	Slug          string                 `json:"slug"`
	Status        string                 `json:"status"`
	Type          string                 `json:"type"`
	Link          string                 `json:"link"`
	Title         RenderedContent        `json:"title"`
	Content       RenderedContent        `json:"content"`
	Excerpt       RenderedContent        `json:"excerpt"`
	Author        int                    `json:"author"`
	FeaturedMedia int                    `json:"featured_media"`
	CommentStatus string                 `json:"comment_status"`
	PingStatus    string                 `json:"ping_status"`
	Sticky        bool                   `json:"sticky"`
	Template      string                 `json:"template"`
	Format        string                 `json:"format"`
	Meta          map[string]interface{} `json:"meta"`
	Categories    []int                  `json:"categories"`
	Tags          []int                  `json:"tags"`
	Links         Links                  `json:"_links"`
	SEO           SEOData                `json:"seo,omitempty"`
}

// WordPressMedia represents a WordPress media item
type WordPressMedia struct {
	ID            int             `json:"id"`
	Date          WordPressTime   `json:"date"`
	DateGMT       WordPressTime   `json:"date_gmt"`
	GUID          GUID            `json:"guid"`
	Modified      WordPressTime   `json:"modified"`
	ModifiedGMT   WordPressTime   `json:"modified_gmt"`
	Slug          string          `json:"slug"`
	Status        string          `json:"status"`
	Type          string          `json:"type"`
	Link          string          `json:"link"`
	Title         RenderedContent `json:"title"`
	Author        int             `json:"author"`
	CommentStatus string          `json:"comment_status"`
	PingStatus    string          `json:"ping_status"`
	Template      string          `json:"template"`
	Meta          interface{}     `json:"meta"`
	Description   RenderedContent `json:"description"`
	Caption       RenderedContent `json:"caption"`
	AltText       string          `json:"alt_text"`
	MediaType     string          `json:"media_type"`
	MimeType      string          `json:"mime_type"`
	MediaDetails  MediaDetails    `json:"media_details"`
	Post          int             `json:"post"`
	SourceURL     string          `json:"source_url"`
	Links         Links           `json:"_links"`
}

// WordPressCategory represents a WordPress category
type WordPressCategory struct {
	ID          int         `json:"id"`
	Count       int         `json:"count"`
	Description string      `json:"description"`
	Link        string      `json:"link"`
	Name        string      `json:"name"`
	Slug        string      `json:"slug"`
	Taxonomy    string      `json:"taxonomy"`
	Parent      int         `json:"parent"`
	Meta        interface{} `json:"meta"` // Can be array or object depending on WP config
	Links       Links       `json:"_links"`
}

// WordPressTag represents a WordPress tag
type WordPressTag struct {
	ID          int         `json:"id"`
	Count       int         `json:"count"`
	Description string      `json:"description"`
	Link        string      `json:"link"`
	Name        string      `json:"name"`
	Slug        string      `json:"slug"`
	Taxonomy    string      `json:"taxonomy"`
	Meta        interface{} `json:"meta"` // Can be array or object depending on WP config
	Links       Links       `json:"_links"`
}

// WordPressUser represents a WordPress user
type WordPressUser struct {
	ID          int               `json:"id"`
	Name        string            `json:"name"`
	URL         string            `json:"url"`
	Description string            `json:"description"`
	Link        string            `json:"link"`
	Slug        string            `json:"slug"`
	AvatarURLs  map[string]string `json:"avatar_urls"`
	Meta        interface{}       `json:"meta"` // Can be array or object depending on WP config
	Links       Links             `json:"_links"`
}

// RenderedContent represents rendered WordPress content
type RenderedContent struct {
	Rendered  string `json:"rendered"`
	Protected bool   `json:"protected,omitempty"`
}

// GUID represents a WordPress GUID
type GUID struct {
	Rendered string `json:"rendered"`
}

// MediaDetails represents WordPress media details
type MediaDetails struct {
	Width     interface{}            `json:"width,omitempty"`
	Height    interface{}            `json:"height,omitempty"`
	File      string                 `json:"file,omitempty"`
	Sizes     map[string]MediaSize   `json:"sizes,omitempty"`
	ImageMeta map[string]interface{} `json:"image_meta,omitempty"`
	Length    interface{}            `json:"length,omitempty"`
	Filesize  interface{}            `json:"filesize,omitempty"`
}

// MediaSize represents a WordPress media size
type MediaSize struct {
	File      string      `json:"file"`
	Width     interface{} `json:"width"`
	Height    interface{} `json:"height"`
	MimeType  string      `json:"mime_type"`
	SourceURL string      `json:"source_url"`
}

// Links represents WordPress API links
type Links struct {
	Self               []Link `json:"self,omitempty"`
	Collection         []Link `json:"collection,omitempty"`
	About              []Link `json:"about,omitempty"`
	Author             []Link `json:"author,omitempty"`
	Replies            []Link `json:"replies,omitempty"`
	VersionHistory     []Link `json:"version-history,omitempty"`
	PredecessorVersion []Link `json:"predecessor-version,omitempty"`
	WPFeaturedmedia    []Link `json:"wp:featuredmedia,omitempty"`
	WPAttachment       []Link `json:"wp:attachment,omitempty"`
	WPTerm             []Link `json:"wp:term,omitempty"`
	Curies             []Link `json:"curies,omitempty"`
}

// Link represents a WordPress API link
type Link struct {
	Href string `json:"href"`
}

// ExportData represents the complete export data structure
type ExportData struct {
	Site       SiteInfo             `json:"site"`
	Posts      []WordPressPost      `json:"posts"`
	Pages      []WordPressPost      `json:"pages"`
	Products   []WooCommerceProduct `json:"products,omitempty"`
	Media      []WordPressMedia     `json:"media"`
	Categories []WordPressCategory  `json:"categories"`
	Tags       []WordPressTag       `json:"tags"`
	Users      []WordPressUser      `json:"users"`
	Analytics  *Analytics           `json:"analytics,omitempty"`
	ExportedAt time.Time            `json:"exported_at"`
	Stats      ExportStats          `json:"stats"`
}

// SiteInfo represents WordPress site information
type SiteInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	URL         string `json:"url"`
	HomeURL     string `json:"home_url"`
	AdminEmail  string `json:"admin_email"`
	Timezone    string `json:"timezone"`
	DateFormat  string `json:"date_format"`
	TimeFormat  string `json:"time_format"`
	StartOfWeek int    `json:"start_of_week"`
	Language    string `json:"language"`
}

// ExportStats represents export statistics
type ExportStats struct {
	TotalPosts      int `json:"total_posts"`
	TotalPages      int `json:"total_pages"`
	TotalProducts   int `json:"total_products"`
	TotalMedia      int `json:"total_media"`
	TotalCategories int `json:"total_categories"`
	TotalTags       int `json:"total_tags"`
	TotalUsers      int `json:"total_users"`
	MediaDownloaded int `json:"media_downloaded"`
	BruteForceFound int `json:"brute_force_found"`
}

// WooCommerceProduct represents a WooCommerce product
type WooCommerceProduct struct {
	ID                int                `json:"id"`
	Name              string             `json:"name"`
	Slug              string             `json:"slug"`
	Permalink         string             `json:"permalink"`
	DateCreated       WordPressTime      `json:"date_created"`
	DateModified      WordPressTime      `json:"date_modified"`
	Type              string             `json:"type"`
	Status            string             `json:"status"`
	Featured          bool               `json:"featured"`
	CatalogVisibility string             `json:"catalog_visibility"`
	Description       string             `json:"description"`
	ShortDescription  string             `json:"short_description"`
	SKU               string             `json:"sku"`
	Price             string             `json:"price"`
	RegularPrice      string             `json:"regular_price"`
	SalePrice         string             `json:"sale_price"`
	OnSale            bool               `json:"on_sale"`
	Purchasable       bool               `json:"purchasable"`
	TotalSales        int                `json:"total_sales"`
	Virtual           bool               `json:"virtual"`
	Downloadable      bool               `json:"downloadable"`
	TaxStatus         string             `json:"tax_status"`
	TaxClass          string             `json:"tax_class"`
	ManageStock       bool               `json:"manage_stock"`
	StockQuantity     interface{}        `json:"stock_quantity"`
	StockStatus       string             `json:"stock_status"`
	Backorders        string             `json:"backorders"`
	BackordersAllowed bool               `json:"backorders_allowed"`
	BackOrdered       bool               `json:"backordered"`
	SoldIndividually  bool               `json:"sold_individually"`
	Weight            string             `json:"weight"`
	Dimensions        ProductDimensions  `json:"dimensions"`
	ShippingRequired  bool               `json:"shipping_required"`
	ShippingTaxable   bool               `json:"shipping_taxable"`
	ShippingClass     string             `json:"shipping_class"`
	ShippingClassID   int                `json:"shipping_class_id"`
	ReviewsAllowed    bool               `json:"reviews_allowed"`
	AverageRating     string             `json:"average_rating"`
	RatingCount       int                `json:"rating_count"`
	ParentID          int                `json:"parent_id"`
	PurchaseNote      string             `json:"purchase_note"`
	Categories        []ProductCategory  `json:"categories"`
	Tags              []ProductTag       `json:"tags"`
	Images            []ProductImage     `json:"images"`
	Attributes        []ProductAttribute `json:"attributes"`
	DefaultAttributes []ProductAttribute `json:"default_attributes"`
	Variations        []int              `json:"variations"`
	MetaData          []ProductMeta      `json:"meta_data"`
}

// ProductDimensions represents product dimensions
type ProductDimensions struct {
	Length string `json:"length"`
	Width  string `json:"width"`
	Height string `json:"height"`
}

// ProductCategory represents a product category
type ProductCategory struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// ProductTag represents a product tag
type ProductTag struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// ProductImage represents a product image
type ProductImage struct {
	ID           int           `json:"id"`
	Src          string        `json:"src"`
	Name         string        `json:"name"`
	Alt          string        `json:"alt"`
	DateCreated  WordPressTime `json:"date_created"`
	DateModified WordPressTime `json:"date_modified"`
}

// ProductAttribute represents a product attribute
type ProductAttribute struct {
	ID        int      `json:"id"`
	Name      string   `json:"name"`
	Position  int      `json:"position"`
	Visible   bool     `json:"visible"`
	Variation bool     `json:"variation"`
	Options   []string `json:"options"`
}

// ProductMeta represents product metadata
type ProductMeta struct {
	ID    int         `json:"id"`
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
}
