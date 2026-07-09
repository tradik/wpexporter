package xmlrpc

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

// maxXMLRPCResponseBytes bounds a single XML-RPC response to guard against
// unbounded allocation from a hostile or misbehaving endpoint (SEC-002).
const maxXMLRPCResponseBytes = 32 << 20 // 32 MiB

// Client represents a WordPress XML-RPC client
type Client struct {
	config   *config.Config
	username string
	password string
	endpoint string
	blogID   int
}

// NewClient creates a new WordPress XML-RPC client
func NewClient(cfg *config.Config, username, password string) (*Client, error) {
	// Validate URL
	if cfg.URL == "" {
		return nil, fmt.Errorf("URL cannot be empty")
	}

	// Parse and validate URL
	parsedURL, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Check for valid scheme
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("URL must have http or https scheme")
	}

	// Check for valid host
	if parsedURL.Host == "" {
		return nil, fmt.Errorf("URL must have a valid host")
	}

	// Construct XML-RPC endpoint
	endpoint := strings.TrimSuffix(parsedURL.String(), "/") + "/xmlrpc.php"

	return &Client{
		config:   cfg,
		username: username,
		password: password,
		endpoint: endpoint,
		blogID:   1, // Default blog ID
	}, nil
}

// XMLRPCRequest represents an XML-RPC request
type XMLRPCRequest struct {
	XMLName xml.Name `xml:"methodCall"`
	Method  string   `xml:"methodName"`
	Params  []Param  `xml:"params>param"`
}

// Param represents an XML-RPC parameter
type Param struct {
	Value Value `xml:"value"`
}

// Value represents an XML-RPC value. All scalar types are optional pointers so
// that an absent type element is distinguishable from an empty one. Raw captures
// character data for the untyped <value>text</value> form.
type Value struct {
	String   *string `xml:"string,omitempty"`
	Int      *int    `xml:"int,omitempty"`
	I4       *int    `xml:"i4,omitempty"`
	Boolean  *string `xml:"boolean,omitempty"`
	Double   *string `xml:"double,omitempty"`
	DateTime *string `xml:"dateTime.iso8601,omitempty"`
	Base64   *string `xml:"base64,omitempty"`
	Struct   *Struct `xml:"struct,omitempty"`
	Array    *Array  `xml:"array,omitempty"`
	Raw      string  `xml:",chardata"`
}

// AsString returns the value coerced to a string, regardless of its XML-RPC type.
func (v *Value) AsString() string {
	switch {
	case v == nil:
		return ""
	case v.String != nil:
		return *v.String
	case v.DateTime != nil:
		return *v.DateTime
	case v.Int != nil:
		return strconv.Itoa(*v.Int)
	case v.I4 != nil:
		return strconv.Itoa(*v.I4)
	case v.Boolean != nil:
		return *v.Boolean
	case v.Double != nil:
		return *v.Double
	case v.Base64 != nil:
		return *v.Base64
	default:
		return strings.TrimSpace(v.Raw)
	}
}

// AsInt returns the value coerced to an int (0 when not parseable).
func (v *Value) AsInt() int {
	if v == nil {
		return 0
	}
	if v.Int != nil {
		return *v.Int
	}
	if v.I4 != nil {
		return *v.I4
	}
	if n, err := strconv.Atoi(strings.TrimSpace(v.AsString())); err == nil {
		return n
	}
	return 0
}

// Struct represents an XML-RPC struct
type Struct struct {
	Members []Member `xml:"member"`
}

// Member represents a struct member
type Member struct {
	Name  string `xml:"name"`
	Value Value  `xml:"value"`
}

// Array represents an XML-RPC array
type Array struct {
	Data []Value `xml:"data>value"`
}

// XMLRPCResponse represents an XML-RPC response
type XMLRPCResponse struct {
	XMLName xml.Name `xml:"methodResponse"`
	Params  []Param  `xml:"params>param,omitempty"`
	Fault   *Fault   `xml:"fault,omitempty"`
}

// Fault represents an XML-RPC fault
type Fault struct {
	Value Value `xml:"value"`
}

// TestConnection tests the XML-RPC connection
func (c *Client) TestConnection() error {
	req := &XMLRPCRequest{
		Method: "wp.getOptions",
		Params: []Param{
			{Value: Value{Int: &c.blogID}},
			{Value: Value{String: &c.username}},
			{Value: Value{String: &c.password}},
		},
	}

	_, err := c.makeRequest(req)
	return err
}

// GetSiteInfo retrieves WordPress site information
func (c *Client) GetSiteInfo() (*models.SiteInfo, error) {
	req := &XMLRPCRequest{
		Method: "wp.getOptions",
		Params: []Param{
			{Value: Value{Int: &c.blogID}},
			{Value: Value{String: &c.username}},
			{Value: Value{String: &c.password}},
		},
	}

	resp, err := c.makeRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get site options: %w", err)
	}

	return c.parseSiteInfo(resp), nil
}

// GetPosts retrieves all posts
func (c *Client) GetPosts() ([]models.WordPressPost, error) {
	allPosts := make([]models.WordPressPost, 0)
	offset := 0
	limit := 100

	for {
		filter := &Struct{
			Members: []Member{
				{Name: "number", Value: Value{Int: &limit}},
				{Name: "offset", Value: Value{Int: &offset}},
			},
		}

		req := &XMLRPCRequest{
			Method: "wp.getPosts",
			Params: []Param{
				{Value: Value{Int: &c.blogID}},
				{Value: Value{String: &c.username}},
				{Value: Value{String: &c.password}},
				{Value: Value{Struct: filter}},
			},
		}

		resp, err := c.makeRequest(req)
		if err != nil {
			return nil, fmt.Errorf("failed to get posts: %w", err)
		}

		posts := c.parsePostsResponse(resp)
		if len(posts) == 0 {
			break
		}

		allPosts = append(allPosts, posts...)
		offset += limit

		if len(posts) < limit {
			break
		}
	}

	return allPosts, nil
}

// GetPages retrieves all pages
func (c *Client) GetPages() ([]models.WordPressPost, error) {
	allPages := make([]models.WordPressPost, 0)
	offset := 0
	limit := 100

	for {
		filter := &Struct{
			Members: []Member{
				{Name: "number", Value: Value{Int: &limit}},
				{Name: "offset", Value: Value{Int: &offset}},
			},
		}

		req := &XMLRPCRequest{
			Method: "wp.getPages",
			Params: []Param{
				{Value: Value{Int: &c.blogID}},
				{Value: Value{String: &c.username}},
				{Value: Value{String: &c.password}},
				{Value: Value{Struct: filter}},
			},
		}

		resp, err := c.makeRequest(req)
		if err != nil {
			return nil, fmt.Errorf("failed to get pages: %w", err)
		}

		pages := c.parsePostsResponse(resp)
		if len(pages) == 0 {
			break
		}

		allPages = append(allPages, pages...)
		offset += limit

		if len(pages) < limit {
			break
		}
	}

	return allPages, nil
}

// GetMedia retrieves all media items
func (c *Client) GetMedia() ([]models.WordPressMedia, error) {
	allMedia := make([]models.WordPressMedia, 0)
	offset := 0
	limit := 100

	for {
		filter := &Struct{
			Members: []Member{
				{Name: "number", Value: Value{Int: &limit}},
				{Name: "offset", Value: Value{Int: &offset}},
			},
		}

		req := &XMLRPCRequest{
			Method: "wp.getMediaLibrary",
			Params: []Param{
				{Value: Value{Int: &c.blogID}},
				{Value: Value{String: &c.username}},
				{Value: Value{String: &c.password}},
				{Value: Value{Struct: filter}},
			},
		}

		resp, err := c.makeRequest(req)
		if err != nil {
			return nil, fmt.Errorf("failed to get media: %w", err)
		}

		media := c.parseMediaResponse(resp)
		if len(media) == 0 {
			break
		}

		allMedia = append(allMedia, media...)
		offset += limit

		if len(media) < limit {
			break
		}
	}

	return allMedia, nil
}

// GetCategories retrieves all categories
func (c *Client) GetCategories() ([]models.WordPressCategory, error) {
	req := &XMLRPCRequest{
		Method: "wp.getTerms",
		Params: []Param{
			{Value: Value{Int: &c.blogID}},
			{Value: Value{String: &c.username}},
			{Value: Value{String: &c.password}},
			{Value: Value{String: stringPtr("category")}},
		},
	}

	resp, err := c.makeRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get categories: %w", err)
	}

	return c.parseCategoriesResponse(resp), nil
}

// GetTags retrieves all tags
func (c *Client) GetTags() ([]models.WordPressTag, error) {
	req := &XMLRPCRequest{
		Method: "wp.getTerms",
		Params: []Param{
			{Value: Value{Int: &c.blogID}},
			{Value: Value{String: &c.username}},
			{Value: Value{String: &c.password}},
			{Value: Value{String: stringPtr("post_tag")}},
		},
	}

	resp, err := c.makeRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get tags: %w", err)
	}

	return c.parseTagsResponse(resp), nil
}

// GetUsers retrieves all users
func (c *Client) GetUsers() ([]models.WordPressUser, error) {
	req := &XMLRPCRequest{
		Method: "wp.getUsers",
		Params: []Param{
			{Value: Value{Int: &c.blogID}},
			{Value: Value{String: &c.username}},
			{Value: Value{String: &c.password}},
		},
	}

	resp, err := c.makeRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get users: %w", err)
	}

	return c.parseUsersResponse(resp), nil
}

// makeRequest makes an XML-RPC request
func (c *Client) makeRequest(req *XMLRPCRequest) (*XMLRPCResponse, error) {
	// Marshal request to XML
	xmlData, err := xml.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal XML-RPC request: %w", err)
	}

	// Add XML declaration
	xmlRequest := []byte(`<?xml version="1.0" encoding="UTF-8"?>` + "\n" + string(xmlData))

	// Create HTTP request
	httpReq, err := http.NewRequest("POST", c.endpoint, bytes.NewBuffer(xmlRequest))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "text/xml")
	httpReq.Header.Set("User-Agent", c.config.UserAgent)

	// Make HTTP request
	client := &http.Client{
		Timeout: time.Duration(c.config.Timeout) * time.Second,
	}

	httpResp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer func() {
		_ = httpResp.Body.Close()
	}()

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request failed with status: %d", httpResp.StatusCode)
	}

	// Read response body, bounded to guard against unbounded allocation (SEC-002).
	body, err := io.ReadAll(io.LimitReader(httpResp.Body, maxXMLRPCResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse XML-RPC response
	var resp XMLRPCResponse
	if err := xml.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse XML-RPC response: %w", err)
	}

	// Check for fault
	if resp.Fault != nil {
		return nil, fmt.Errorf("XML-RPC fault occurred")
	}

	return &resp, nil
}

// responseArray returns the array of values from the first response parameter,
// or nil if the response is not an array (e.g. empty params or a scalar).
func responseArray(resp *XMLRPCResponse) []Value {
	if resp == nil || len(resp.Params) == 0 {
		return nil
	}
	if arr := resp.Params[0].Value.Array; arr != nil {
		return arr.Data
	}
	return nil
}

// structToMap indexes the members of an XML-RPC struct by name for O(1) lookup.
func structToMap(s *Struct) map[string]*Value {
	m := make(map[string]*Value)
	if s == nil {
		return m
	}
	for i := range s.Members {
		m[s.Members[i].Name] = &s.Members[i].Value
	}
	return m
}

// mapStructs applies fn to every struct element of the response array and returns
// the collected results. It centralizes the array/struct iteration boilerplate so
// each parser only declares its own field mapping.
func mapStructs[T any](resp *XMLRPCResponse, fn func(m map[string]*Value) T) []T {
	out := make([]T, 0)
	for _, item := range responseArray(resp) {
		if item.Struct == nil {
			continue
		}
		out = append(out, fn(structToMap(item.Struct)))
	}
	return out
}

// valStr returns the string value of a named struct member (empty if absent).
func valStr(m map[string]*Value, key string) string {
	if v, ok := m[key]; ok {
		return v.AsString()
	}
	return ""
}

// valInt returns the int value of a named struct member (0 if absent).
func valInt(m map[string]*Value, key string) int {
	if v, ok := m[key]; ok {
		return v.AsInt()
	}
	return 0
}

// parseXMLRPCTime parses a WordPress dateTime.iso8601 value (and common fallbacks).
func parseXMLRPCTime(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	formats := []string{
		"20060102T15:04:05", // WordPress dateTime.iso8601 (basic date)
		"2006-01-02T15:04:05",
		time.RFC3339,
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// parsePostsResponse maps a wp.getPosts/wp.getPages struct array into posts.
func (c *Client) parsePostsResponse(resp *XMLRPCResponse) []models.WordPressPost {
	return mapStructs(resp, func(m map[string]*Value) models.WordPressPost {
		return models.WordPressPost{
			ID:       valInt(m, "post_id"),
			Slug:     valStr(m, "post_name"),
			Status:   valStr(m, "post_status"),
			Type:     valStr(m, "post_type"),
			Link:     valStr(m, "link"),
			Author:   valInt(m, "post_author"),
			Title:    models.RenderedContent{Rendered: valStr(m, "post_title")},
			Content:  models.RenderedContent{Rendered: valStr(m, "post_content")},
			Excerpt:  models.RenderedContent{Rendered: valStr(m, "post_excerpt")},
			Date:     models.WordPressTime{Time: parseXMLRPCTime(valStr(m, "post_date"))},
			Modified: models.WordPressTime{Time: parseXMLRPCTime(valStr(m, "post_modified"))},
		}
	})
}

// parseMediaResponse maps a wp.getMediaLibrary struct array into media items.
func (c *Client) parseMediaResponse(resp *XMLRPCResponse) []models.WordPressMedia {
	return mapStructs(resp, func(m map[string]*Value) models.WordPressMedia {
		link := valStr(m, "link")
		return models.WordPressMedia{
			ID:          valInt(m, "attachment_id"),
			Link:        link,
			SourceURL:   link,
			MimeType:    valStr(m, "type"),
			Title:       models.RenderedContent{Rendered: valStr(m, "title")},
			Caption:     models.RenderedContent{Rendered: valStr(m, "caption")},
			Description: models.RenderedContent{Rendered: valStr(m, "description")},
			Date:        models.WordPressTime{Time: parseXMLRPCTime(valStr(m, "date_created_gmt"))},
		}
	})
}

// parseCategoriesResponse maps a wp.getTerms struct array into categories.
func (c *Client) parseCategoriesResponse(resp *XMLRPCResponse) []models.WordPressCategory {
	return mapStructs(resp, func(m map[string]*Value) models.WordPressCategory {
		return models.WordPressCategory{
			ID:          valInt(m, "term_id"),
			Name:        valStr(m, "name"),
			Slug:        valStr(m, "slug"),
			Description: valStr(m, "description"),
			Taxonomy:    valStr(m, "taxonomy"),
			Parent:      valInt(m, "parent"),
			Count:       valInt(m, "count"),
		}
	})
}

// parseTagsResponse maps a wp.getTerms struct array into tags.
func (c *Client) parseTagsResponse(resp *XMLRPCResponse) []models.WordPressTag {
	return mapStructs(resp, func(m map[string]*Value) models.WordPressTag {
		return models.WordPressTag{
			ID:          valInt(m, "term_id"),
			Name:        valStr(m, "name"),
			Slug:        valStr(m, "slug"),
			Description: valStr(m, "description"),
			Taxonomy:    valStr(m, "taxonomy"),
			Count:       valInt(m, "count"),
		}
	})
}

// parseUsersResponse maps a wp.getUsers struct array into users.
func (c *Client) parseUsersResponse(resp *XMLRPCResponse) []models.WordPressUser {
	return mapStructs(resp, func(m map[string]*Value) models.WordPressUser {
		return models.WordPressUser{
			ID:          valInt(m, "user_id"),
			Name:        valStr(m, "display_name"),
			Slug:        valStr(m, "nicename"),
			URL:         valStr(m, "url"),
			Description: valStr(m, "bio"),
		}
	})
}

// parseSiteInfo maps a wp.getOptions struct response into site information.
// Each option may be either a direct scalar or a nested struct with a "value" member.
func (c *Client) parseSiteInfo(resp *XMLRPCResponse) *models.SiteInfo {
	info := &models.SiteInfo{
		URL:  c.config.URL,
		Name: "WordPress Site (XML-RPC)",
	}
	if resp == nil || len(resp.Params) == 0 || resp.Params[0].Value.Struct == nil {
		return info
	}
	opts := structToMap(resp.Params[0].Value.Struct)
	if name := optionValue(opts, "blog_title"); name != "" {
		info.Name = name
	}
	info.Description = optionValue(opts, "blog_tagline")
	info.HomeURL = optionValue(opts, "home_url")
	if lang := optionValue(opts, "blog_language"); lang != "" {
		info.Language = lang
	}
	if tz := optionValue(opts, "time_zone"); tz != "" {
		info.Timezone = tz
	}
	return info
}

// optionValue extracts an option value that may be a scalar or a {value, desc} struct.
func optionValue(opts map[string]*Value, key string) string {
	v, ok := opts[key]
	if !ok {
		return ""
	}
	if v.Struct != nil {
		return valStr(structToMap(v.Struct), "value")
	}
	return v.AsString()
}

// Helper function to create string pointer
func stringPtr(s string) *string {
	return &s
}
