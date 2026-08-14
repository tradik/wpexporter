package mcp

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetTools(t *testing.T) {
	tools := GetTools()
	assert.NotEmpty(t, tools)

	// Verify expected tools exist
	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	expectedTools := []string{
		"list_formats",
		"get_site_info",
		"list_posts",
		"list_pages",
		"export_site",
		"get_post",
		"list_categories",
		"list_media",
	}

	for _, name := range expectedTools {
		assert.True(t, toolNames[name], "Expected tool %s not found", name)
	}
}

func TestToolSchemas(t *testing.T) {
	tools := GetTools()

	for _, tool := range tools {
		assert.NotEmpty(t, tool.Name)
		assert.NotEmpty(t, tool.Description)
		assert.Equal(t, "object", tool.InputSchema.Type)
	}
}

func TestHandleListFormats(t *testing.T) {
	result, err := handleListFormats(nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Len(t, result.Content, 1)
	assert.False(t, result.IsError)

	// Verify JSON is valid
	var formats []map[string]string
	err = json.Unmarshal([]byte(result.Content[0].Text), &formats)
	require.NoError(t, err)
	assert.NotEmpty(t, formats)

	// Check expected formats
	formatNames := make(map[string]bool)
	for _, f := range formats {
		formatNames[f["name"]] = true
	}

	expectedFormats := []string{"json", "markdown", "shopify", "magento", "wordpress", "drupal", "ghost", "strapi", "contentful"}
	for _, name := range expectedFormats {
		assert.True(t, formatNames[name], "Expected format %s not found", name)
	}
}

func TestConfigFromArgs(t *testing.T) {
	args := map[string]interface{}{
		"url":           "https://example.com",
		"authUser":      "admin",
		"authPass":      "secret",
		"authToken":     "token123",
		"format":        "markdown",
		"output":        "/tmp/export",
		"downloadMedia": true,
		"noPosts":       true,
		"noPages":       false,
		"noProducts":    true,
		"noComments":    true,
		"pathFilter":    "/blog/",
		"assistedCrawl": true,
		"flatHtml":      true,
	}

	cfg := configFromArgs(args)

	assert.Equal(t, "https://example.com", cfg.URL)
	assert.Equal(t, "admin", cfg.AuthUser)
	assert.Equal(t, "secret", cfg.AuthPass)
	assert.Equal(t, "token123", cfg.AuthToken)
	assert.Equal(t, "markdown", cfg.Format)
	assert.Equal(t, "/tmp/export", cfg.Output)
	assert.True(t, cfg.DownloadMedia)
	assert.True(t, cfg.NoPosts)
	assert.False(t, cfg.NoPages)
	assert.True(t, cfg.NoProducts)
	assert.True(t, cfg.NoComments)
	assert.Equal(t, "/blog/", cfg.PathFilter)
	assert.True(t, cfg.AssistedCrawl)
	assert.True(t, cfg.FlatHTML)
	// MCP config optimizations
	assert.Equal(t, 10, cfg.Timeout)
	assert.Equal(t, 1, cfg.Retries)
}

func TestConfigFromArgsDefaults(t *testing.T) {
	cfg := configFromArgs(map[string]interface{}{})

	assert.Empty(t, cfg.URL)
	assert.Empty(t, cfg.AuthUser)
	assert.Equal(t, "json", cfg.Format)
	assert.True(t, cfg.DownloadMedia)
	assert.False(t, cfg.NoPosts)
	// MCP specific defaults
	assert.Equal(t, 10, cfg.Timeout)
	assert.Equal(t, 1, cfg.Retries)
}

func TestHandleGetPostMissingID(t *testing.T) {
	args := map[string]interface{}{
		"url": "https://example.com",
	}

	result, err := handleGetPost(args)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "postId is required")
}

func TestRegisterAllTools(t *testing.T) {
	server := NewServer("test", "1.0")
	RegisterAllTools(server)

	server.mu.RLock()
	defer server.mu.RUnlock()

	expectedTools := []string{
		"list_formats",
		"get_site_info",
		"list_posts",
		"list_pages",
		"export_site",
		"get_post",
		"list_categories",
		"list_media",
	}

	for _, name := range expectedTools {
		_, exists := server.tools[name]
		assert.True(t, exists, "Handler for %s not registered", name)
	}
}

func TestToolRequiredFields(t *testing.T) {
	tools := GetTools()

	toolsWithURLRequired := []string{
		"get_site_info",
		"list_posts",
		"list_pages",
		"export_site",
		"get_post",
		"list_categories",
		"list_media",
	}

	for _, tool := range tools {
		for _, name := range toolsWithURLRequired {
			if tool.Name == name {
				assert.Contains(t, tool.InputSchema.Required, "url",
					"Tool %s should require 'url' field", name)
			}
		}
	}
}

func TestExportSiteToolSchema(t *testing.T) {
	tools := GetTools()

	var exportTool *Tool
	for i, tool := range tools {
		if tool.Name == "export_site" {
			exportTool = &tools[i]
			break
		}
	}

	require.NotNil(t, exportTool)

	// Check format enum
	formatProp := exportTool.InputSchema.Properties["format"]
	assert.NotEmpty(t, formatProp.Enum)
	assert.Contains(t, formatProp.Enum, "json")
	assert.Contains(t, formatProp.Enum, "markdown")
	assert.Contains(t, formatProp.Enum, "shopify")
}

func TestGetPostToolSchema(t *testing.T) {
	tools := GetTools()

	var postTool *Tool
	for i, tool := range tools {
		if tool.Name == "get_post" {
			postTool = &tools[i]
			break
		}
	}

	require.NotNil(t, postTool)
	assert.Contains(t, postTool.InputSchema.Required, "url")
	assert.Contains(t, postTool.InputSchema.Required, "postId")
}

func TestListPostsToolSchema(t *testing.T) {
	tools := GetTools()

	var listTool *Tool
	for i, tool := range tools {
		if tool.Name == "list_posts" {
			listTool = &tools[i]
			break
		}
	}

	require.NotNil(t, listTool)

	// Check limit property exists
	limitProp, exists := listTool.InputSchema.Properties["limit"]
	assert.True(t, exists)
	assert.Equal(t, "integer", limitProp.Type)
	assert.Equal(t, 10, limitProp.Default)

	// Check pathFilter property exists
	_, exists = listTool.InputSchema.Properties["pathFilter"]
	assert.True(t, exists)
}

func TestListMediaToolSchema(t *testing.T) {
	tools := GetTools()

	var mediaTool *Tool
	for i, tool := range tools {
		if tool.Name == "list_media" {
			mediaTool = &tools[i]
			break
		}
	}

	require.NotNil(t, mediaTool)

	// Check limit property
	limitProp := mediaTool.InputSchema.Properties["limit"]
	assert.Equal(t, "integer", limitProp.Type)
	assert.Equal(t, 20, limitProp.Default)
}

func TestListFormatsToolSchema(t *testing.T) {
	tools := GetTools()

	var formatsTool *Tool
	for i, tool := range tools {
		if tool.Name == "list_formats" {
			formatsTool = &tools[i]
			break
		}
	}

	require.NotNil(t, formatsTool)
	assert.Empty(t, formatsTool.InputSchema.Required)
	assert.Empty(t, formatsTool.InputSchema.Properties)
}

func TestToolDescriptions(t *testing.T) {
	tools := GetTools()

	for _, tool := range tools {
		assert.NotEmpty(t, tool.Description, "Tool %s should have a description", tool.Name)
		assert.Greater(t, len(tool.Description), 10, "Tool %s description too short", tool.Name)
	}
}

func TestAllToolsHaveHandlers(t *testing.T) {
	server := NewServer("test", "1.0")
	RegisterAllTools(server)

	tools := GetTools()

	server.mu.RLock()
	defer server.mu.RUnlock()

	for _, tool := range tools {
		_, exists := server.tools[tool.Name]
		assert.True(t, exists, "Tool %s has no registered handler", tool.Name)
	}
}

func TestHandleListFormatsOutput(t *testing.T) {
	result, err := handleListFormats(nil)
	require.NoError(t, err)

	// Check content type
	assert.Equal(t, "text", result.Content[0].Type)

	// Verify all 15 formats are present
	var formats []map[string]string
	err = json.Unmarshal([]byte(result.Content[0].Text), &formats)
	require.NoError(t, err)
	assert.Len(t, formats, 15, "Should have 15 export formats")

	// The list and the export_site enum must not drift apart.
	names := make([]string, 0, len(formats))
	for _, f := range formats {
		names = append(names, f["name"])
	}
	assert.Contains(t, names, "ssg", "ssg format should be advertised")

	// Verify each format has name and description
	for _, f := range formats {
		assert.NotEmpty(t, f["name"], "Format should have name")
		assert.NotEmpty(t, f["description"], "Format should have description")
	}
}

func TestAuthenticationFields(t *testing.T) {
	tools := GetTools()

	toolsWithAuth := []string{
		"get_site_info",
		"list_posts",
		"list_pages",
		"export_site",
		"get_post",
		"list_categories",
		"list_media",
	}

	for _, tool := range tools {
		for _, name := range toolsWithAuth {
			if tool.Name == name {
				_, hasAuthUser := tool.InputSchema.Properties["authUser"]
				_, hasAuthPass := tool.InputSchema.Properties["authPass"]
				_, hasAuthToken := tool.InputSchema.Properties["authToken"]

				assert.True(t, hasAuthUser, "Tool %s should have authUser property", name)
				assert.True(t, hasAuthPass, "Tool %s should have authPass property", name)
				assert.True(t, hasAuthToken, "Tool %s should have authToken property", name)
			}
		}
	}
}

func TestExportSiteOptions(t *testing.T) {
	tools := GetTools()

	var exportTool *Tool
	for i, tool := range tools {
		if tool.Name == "export_site" {
			exportTool = &tools[i]
			break
		}
	}

	require.NotNil(t, exportTool)

	// Verify all export options are available
	expectedProps := []string{
		"url", "format", "output", "authUser", "authPass", "authToken",
		"downloadMedia", "noPosts", "noPages", "noProducts", "noComments",
		"pathFilter", "assistedCrawl", "flatHtml",
	}

	for _, prop := range expectedProps {
		_, exists := exportTool.InputSchema.Properties[prop]
		assert.True(t, exists, "Export tool should have %s property", prop)
	}
}

func TestToolCount(t *testing.T) {
	tools := GetTools()
	assert.Len(t, tools, 8, "Should have exactly 8 MCP tools")
}
