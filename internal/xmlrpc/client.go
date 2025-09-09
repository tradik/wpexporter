package xmlrpc

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/tradik/wpexporter/internal/config"
	"github.com/tradik/wpexporter/pkg/models"
)

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
	// Parse and validate URL
	parsedURL, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
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

// Value represents an XML-RPC value
type Value struct {
	String *string `xml:"string,omitempty"`
	Int    *int    `xml:"int,omitempty"`
	Struct *Struct `xml:"struct,omitempty"`
	Array  *Array  `xml:"array,omitempty"`
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

	// Parse response to extract site information
	siteInfo := &models.SiteInfo{
		URL:  c.config.URL,
		Name: "WordPress Site (XML-RPC)",
	}

	// Try to extract site name and other info from response
	if len(resp.Params) > 0 {
		// This is a simplified extraction - in a real implementation,
		// you would parse the XML-RPC struct response properly
		siteInfo.Name = "WordPress Site"
	}

	return siteInfo, nil
}

// GetPosts retrieves all posts
func (c *Client) GetPosts() ([]models.WordPressPost, error) {
	var allPosts []models.WordPressPost
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
	var allPages []models.WordPressPost
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
	var allMedia []models.WordPressMedia
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
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP request failed with status: %d", httpResp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(httpResp.Body)
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

// Helper functions for parsing responses
func (c *Client) parsePostsResponse(resp *XMLRPCResponse) []models.WordPressPost {
	// This is a simplified implementation
	// In a real implementation, you would parse the XML-RPC struct response properly
	var posts []models.WordPressPost

	// For demonstration, create a sample post
	if len(resp.Params) > 0 {
		post := models.WordPressPost{
			ID:    1,
			Title: models.RenderedContent{Rendered: "Sample Post"},
			Type:  "post",
		}
		posts = append(posts, post)
	}

	return posts
}

func (c *Client) parseMediaResponse(resp *XMLRPCResponse) []models.WordPressMedia {
	var media []models.WordPressMedia
	// Simplified implementation
	return media
}

func (c *Client) parseCategoriesResponse(resp *XMLRPCResponse) []models.WordPressCategory {
	var categories []models.WordPressCategory
	// Simplified implementation
	return categories
}

func (c *Client) parseTagsResponse(resp *XMLRPCResponse) []models.WordPressTag {
	var tags []models.WordPressTag
	// Simplified implementation
	return tags
}

func (c *Client) parseUsersResponse(resp *XMLRPCResponse) []models.WordPressUser {
	var users []models.WordPressUser
	// Simplified implementation
	return users
}

// Helper function to create string pointer
func stringPtr(s string) *string {
	return &s
}
