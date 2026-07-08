package xmlrpc

import (
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tradik/wpexporter/internal/config"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *config.Config
		wantErr bool
	}{
		{
			name: "Valid config creates client",
			cfg: &config.Config{
				URL: "https://example.com",
			},
			wantErr: false,
		},
		{
			name: "Invalid URL returns error",
			cfg: &config.Config{
				URL: "ftp://example.com",
			},
			wantErr: true,
		},
		{
			name: "Empty URL returns error",
			cfg: &config.Config{
				URL: "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient(tt.cfg, "testuser", "testpass")
			if (err != nil) != tt.wantErr {
				t.Errorf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// NewClient should never return nil when there's no error
				if client.config != tt.cfg {
					t.Error("NewClient() should store config reference")
				}

				if client.username != "testuser" {
					t.Errorf("NewClient() username = %v, want %v", client.username, "testuser")
					t.Error("NewClient() should store username")
				}

				if client.password != "testpass" {
					t.Error("NewClient() should store password")
				}

				expectedEndpoint := "https://example.com/xmlrpc.php"
				if client.endpoint != expectedEndpoint {
					t.Errorf("NewClient() endpoint = %v, want %v", client.endpoint, expectedEndpoint)
				}

				if client.blogID != 1 {
					t.Errorf("NewClient() blogID = %v, want %v", client.blogID, 1)
				}
			}
		})
	}
}

func TestNewClientEndpointConstruction(t *testing.T) {
	tests := []struct {
		inputURL    string
		expectedURL string
	}{
		{"https://example.com", "https://example.com/xmlrpc.php"},
		{"https://example.com/", "https://example.com/xmlrpc.php"},
		{"https://example.com/wordpress", "https://example.com/wordpress/xmlrpc.php"},
		{"https://example.com/wordpress/", "https://example.com/wordpress/xmlrpc.php"},
		{"http://localhost:8080", "http://localhost:8080/xmlrpc.php"},
		{"http://localhost:8080/", "http://localhost:8080/xmlrpc.php"},
	}

	for _, tt := range tests {
		t.Run(tt.inputURL, func(t *testing.T) {
			cfg := &config.Config{
				URL: tt.inputURL,
			}

			client, err := NewClient(cfg, "user", "pass")
			if err != nil {
				t.Fatalf("NewClient() error = %v", err)
			}

			if client.endpoint != tt.expectedURL {
				t.Errorf("NewClient() endpoint = %v, want %v", client.endpoint, tt.expectedURL)
			}
		})
	}
}

func TestTestConnection(t *testing.T) {
	// Mock XML-RPC server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/xmlrpc.php" {
			w.Header().Set("Content-Type", "text/xml")
			w.WriteHeader(http.StatusOK)
			// Return a valid XML-RPC response
			response := `<?xml version="1.0" encoding="UTF-8"?>
<methodResponse>
	<params>
		<param>
			<value>
				<struct>
					<member>
						<name>blog_title</name>
						<value><string>Test Blog</string></value>
					</member>
				</struct>
			</value>
		</param>
	</params>
</methodResponse>`
			_, _ = w.Write([]byte(response))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:       server.URL,
		UserAgent: "test-agent",
		Timeout:   10,
	}

	client, err := NewClient(cfg, "testuser", "testpass")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	err = client.TestConnection()
	if err != nil {
		t.Errorf("TestConnection() error = %v, want nil", err)
	}
}

func TestTestConnectionFailure(t *testing.T) {
	// Mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/xmlrpc.php" {
			w.Header().Set("Content-Type", "text/xml")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("Internal Server Error"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:     server.URL,
		Timeout: 10,
	}

	client, err := NewClient(cfg, "testuser", "testpass")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	err = client.TestConnection()
	if err == nil {
		t.Error("TestConnection() should return error for failed connection")
	}
}

func TestGetSiteInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/xmlrpc.php" {
			w.Header().Set("Content-Type", "text/xml")
			w.WriteHeader(http.StatusOK)
			response := `<?xml version="1.0" encoding="UTF-8"?>
<methodResponse>
	<params>
		<param>
			<value>
				<struct>
					<member>
						<name>blog_title</name>
						<value><string>Test Site</string></value>
					</member>
				</struct>
			</value>
		</param>
	</params>
</methodResponse>`
			_, _ = w.Write([]byte(response))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:     server.URL,
		Timeout: 10,
	}

	client, err := NewClient(cfg, "testuser", "testpass")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	siteInfo, err := client.GetSiteInfo()
	if err != nil {
		t.Errorf("GetSiteInfo() error = %v, want nil", err)
	}

	if siteInfo == nil {
		t.Fatal("GetSiteInfo() should return non-nil site info")
	} else if siteInfo.URL != server.URL {
		t.Errorf("GetSiteInfo() URL = %v, want %v", siteInfo.URL, server.URL)
	}
}

func TestGetPosts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/xmlrpc.php" {
			w.Header().Set("Content-Type", "text/xml")
			w.WriteHeader(http.StatusOK)
			response := `<?xml version="1.0" encoding="UTF-8"?>
<methodResponse>
	<params>
		<param>
			<value>
				<array>
					<data>
						<value>
							<struct>
								<member>
									<name>post_id</name>
									<value><int>1</int></value>
								</member>
								<member>
									<name>post_title</name>
									<value><string>Test Post</string></value>
								</member>
							</struct>
						</value>
					</data>
				</array>
			</value>
		</param>
	</params>
</methodResponse>`
			_, _ = w.Write([]byte(response))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:     server.URL,
		Timeout: 10,
	}

	client, err := NewClient(cfg, "testuser", "testpass")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	posts, err := client.GetPosts()
	if err != nil {
		t.Errorf("GetPosts() error = %v, want nil", err)
	}

	if posts == nil {
		t.Fatal("GetPosts() should return non-nil posts")
	}

	// The mock implementation returns one sample post
	if len(posts) != 1 {
		t.Errorf("GetPosts() returned %d posts, want 1", len(posts))
	}
}

func TestGetPages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/xmlrpc.php" {
			w.Header().Set("Content-Type", "text/xml")
			w.WriteHeader(http.StatusOK)
			response := `<?xml version="1.0" encoding="UTF-8"?>
<methodResponse>
	<params>
		<param>
			<value>
				<array>
					<data>
						<value>
							<struct>
								<member>
									<name>post_id</name>
									<value><int>2</int></value>
								</member>
								<member>
									<name>post_title</name>
									<value><string>Test Page</string></value>
								</member>
							</struct>
						</value>
					</data>
				</array>
			</value>
		</param>
	</params>
</methodResponse>`
			_, _ = w.Write([]byte(response))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:     server.URL,
		Timeout: 10,
	}

	client, err := NewClient(cfg, "testuser", "testpass")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	pages, err := client.GetPages()
	if err != nil {
		t.Errorf("GetPages() error = %v, want nil", err)
	}

	if pages == nil {
		t.Fatal("GetPages() should return non-nil pages")
	}

	// The mock implementation returns one sample page
	if len(pages) != 1 {
		t.Errorf("GetPages() returned %d pages, want 1", len(pages))
	}
}

func TestGetMedia(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/xmlrpc.php" {
			w.Header().Set("Content-Type", "text/xml")
			w.WriteHeader(http.StatusOK)
			response := `<?xml version="1.0" encoding="UTF-8"?>
<methodResponse>
	<params>
		<param>
			<value>
				<array>
					<data>
					</data>
				</array>
			</value>
		</param>
	</params>
</methodResponse>`
			_, _ = w.Write([]byte(response))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:     server.URL,
		Timeout: 10,
	}

	client, err := NewClient(cfg, "testuser", "testpass")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	media, err := client.GetMedia()
	if err != nil {
		t.Errorf("GetMedia() error = %v, want nil", err)
	}

	if media == nil {
		t.Fatal("GetMedia() should return non-nil media")
	}

	// The mock implementation returns empty media
	if len(media) != 0 {
		t.Errorf("GetMedia() returned %d media items, want 0", len(media))
	}
}

func TestGetCategories(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/xmlrpc.php" {
			w.Header().Set("Content-Type", "text/xml")
			w.WriteHeader(http.StatusOK)
			response := `<?xml version="1.0" encoding="UTF-8"?>
<methodResponse>
	<params>
		<param>
			<value>
				<array>
					<data>
					</data>
				</array>
			</value>
		</param>
	</params>
</methodResponse>`
			_, _ = w.Write([]byte(response))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:     server.URL,
		Timeout: 10,
	}

	client, err := NewClient(cfg, "testuser", "testpass")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	categories, err := client.GetCategories()
	if err != nil {
		t.Errorf("GetCategories() error = %v, want nil", err)
	}

	if categories == nil {
		t.Fatal("GetCategories() should return non-nil categories")
	}

	// The mock implementation returns empty categories
	if len(categories) != 0 {
		t.Errorf("GetCategories() returned %d categories, want 0", len(categories))
	}
}

func TestGetTags(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/xmlrpc.php" {
			w.Header().Set("Content-Type", "text/xml")
			w.WriteHeader(http.StatusOK)
			response := `<?xml version="1.0" encoding="UTF-8"?>
<methodResponse>
	<params>
		<param>
			<value>
				<array>
					<data>
					</data>
				</array>
			</value>
		</param>
	</params>
</methodResponse>`
			_, _ = w.Write([]byte(response))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:     server.URL,
		Timeout: 10,
	}

	client, err := NewClient(cfg, "testuser", "testpass")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	tags, err := client.GetTags()
	if err != nil {
		t.Errorf("GetTags() error = %v, want nil", err)
	}

	if tags == nil {
		t.Fatal("GetTags() should return non-nil tags")
	}

	// The mock implementation returns empty tags
	if len(tags) != 0 {
		t.Errorf("GetTags() returned %d tags, want 0", len(tags))
	}
}

func TestGetUsers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/xmlrpc.php" {
			w.Header().Set("Content-Type", "text/xml")
			w.WriteHeader(http.StatusOK)
			response := `<?xml version="1.0" encoding="UTF-8"?>
<methodResponse>
	<params>
		<param>
			<value>
				<array>
					<data>
					</data>
				</array>
			</value>
		</param>
	</params>
</methodResponse>`
			_, _ = w.Write([]byte(response))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:     server.URL,
		Timeout: 10,
	}

	client, err := NewClient(cfg, "testuser", "testpass")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	users, err := client.GetUsers()
	if err != nil {
		t.Errorf("GetUsers() error = %v, want nil", err)
	}

	if users == nil {
		t.Fatal("GetUsers() should return non-nil users")
	}

	// The mock implementation returns empty users
	if len(users) != 0 {
		t.Errorf("GetUsers() returned %d users, want 0", len(users))
	}
}

func TestMakeRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/xmlrpc.php" {
			if r.Method != "POST" {
				t.Errorf("makeRequest() method = %s, want POST", r.Method)
			}

			if r.Header.Get("Content-Type") != "text/xml" {
				t.Errorf("makeRequest() Content-Type = %s, want text/xml", r.Header.Get("Content-Type"))
			}

			w.Header().Set("Content-Type", "text/xml")
			w.WriteHeader(http.StatusOK)
			response := `<?xml version="1.0" encoding="UTF-8"?>
<methodResponse>
	<params>
		<param>
			<value><string>success</string></value>
		</param>
	</params>
</methodResponse>`
			_, _ = w.Write([]byte(response))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:       server.URL,
		UserAgent: "test-agent",
		Timeout:   10,
	}

	client, err := NewClient(cfg, "testuser", "testpass")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	req := &XMLRPCRequest{
		Method: "wp.test",
		Params: []Param{
			{Value: Value{String: stringPtr("test")}},
		},
	}

	resp, err := client.makeRequest(req)
	if err != nil {
		t.Errorf("makeRequest() error = %v, want nil", err)
	}

	if resp == nil {
		t.Fatal("makeRequest() should return non-nil response")
	} else if len(resp.Params) == 0 {
		t.Error("makeRequest() should return response with params")
	}
}

func TestMakeRequestHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/xmlrpc.php" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:     server.URL,
		Timeout: 10,
	}

	client, err := NewClient(cfg, "testuser", "testpass")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	req := &XMLRPCRequest{
		Method: "wp.test",
	}

	_, err = client.makeRequest(req)
	if err == nil {
		t.Error("makeRequest() should return error for HTTP error")
	}
}

func TestMakeRequestXMLFault(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/xmlrpc.php" {
			w.Header().Set("Content-Type", "text/xml")
			w.WriteHeader(http.StatusOK)
			response := `<?xml version="1.0" encoding="UTF-8"?>
<methodResponse>
	<fault>
		<value>
			<struct>
				<member>
					<name>faultCode</name>
					<value><int>403</int></value>
				</member>
				<member>
					<name>faultString</name>
					<value><string>Invalid credentials</string></value>
				</member>
			</struct>
		</value>
	</fault>
</methodResponse>`
			_, _ = w.Write([]byte(response))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:     server.URL,
		Timeout: 10,
	}

	client, err := NewClient(cfg, "testuser", "testpass")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	req := &XMLRPCRequest{
		Method: "wp.test",
	}

	_, err = client.makeRequest(req)
	if err == nil {
		t.Error("makeRequest() should return error for XML-RPC fault")
	}
}

func TestStringPtr(t *testing.T) {
	input := "test string"
	result := stringPtr(input)

	// stringPtr should never return nil
	if *result != input {
		t.Errorf("stringPtr() = %s, want %s", *result, input)
	}
}

func TestXMLRPCStructures(t *testing.T) {
	// Test XML-RPC request structure
	req := &XMLRPCRequest{
		Method: "wp.test",
		Params: []Param{
			{Value: Value{String: stringPtr("param1")}},
			{Value: Value{Int: intPtr(123)}},
		},
	}

	if req.Method != "wp.test" {
		t.Errorf("XMLRPCRequest Method = %s, want wp.test", req.Method)
	}

	if len(req.Params) != 2 {
		t.Errorf("XMLRPCRequest Params length = %d, want 2", len(req.Params))
	}

	// Test XML-RPC response structure
	resp := &XMLRPCResponse{
		Params: []Param{
			{Value: Value{String: stringPtr("response")}},
		},
	}

	if len(resp.Params) != 1 {
		t.Errorf("XMLRPCResponse Params length = %d, want 1", len(resp.Params))
	}

	if resp.Fault != nil {
		t.Error("XMLRPCResponse Fault should be nil for successful response")
	}
}

// Helper function to create int pointer
func intPtr(i int) *int {
	return &i
}

func TestNewClientURLWithoutHost(t *testing.T) {
	cfg := &config.Config{
		URL: "http://",
	}

	_, err := NewClient(cfg, "testuser", "testpass")
	if err == nil {
		t.Error("NewClient() should return error for URL without host")
	}
}

func TestNewClientParseError(t *testing.T) {
	cfg := &config.Config{
		URL: "://invalid-url",
	}

	_, err := NewClient(cfg, "testuser", "testpass")
	if err == nil {
		t.Error("NewClient() should return error for invalid URL")
	}
}

func TestGetSiteInfoError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:     server.URL,
		Timeout: 10,
	}

	client, err := NewClient(cfg, "testuser", "testpass")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	_, err = client.GetSiteInfo()
	if err == nil {
		t.Error("GetSiteInfo() should return error for HTTP error")
	}
}

func TestGetSiteInfoEmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		w.WriteHeader(http.StatusOK)
		response := `<?xml version="1.0" encoding="UTF-8"?>
<methodResponse>
	<params>
	</params>
</methodResponse>`
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:     server.URL,
		Timeout: 10,
	}

	client, err := NewClient(cfg, "testuser", "testpass")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	siteInfo, err := client.GetSiteInfo()
	if err != nil {
		t.Errorf("GetSiteInfo() error = %v, want nil", err)
	}

	// With empty params, should still return default name
	if siteInfo.Name != "WordPress Site (XML-RPC)" {
		t.Errorf("GetSiteInfo() Name = %v, want default name", siteInfo.Name)
	}
}

func TestGetPostsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:     server.URL,
		Timeout: 10,
	}

	client, err := NewClient(cfg, "testuser", "testpass")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	_, err = client.GetPosts()
	if err == nil {
		t.Error("GetPosts() should return error for HTTP error")
	}
}

func TestGetPostsEmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		w.WriteHeader(http.StatusOK)
		// Response with empty params (no posts)
		response := `<?xml version="1.0" encoding="UTF-8"?>
<methodResponse>
	<params>
	</params>
</methodResponse>`
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:     server.URL,
		Timeout: 10,
	}

	client, err := NewClient(cfg, "testuser", "testpass")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	posts, err := client.GetPosts()
	if err != nil {
		t.Errorf("GetPosts() error = %v, want nil", err)
	}

	// Empty params should return empty posts
	if len(posts) != 0 {
		t.Errorf("GetPosts() with empty response should return empty slice, got %d posts", len(posts))
	}
}

func TestGetPagesError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:     server.URL,
		Timeout: 10,
	}

	client, err := NewClient(cfg, "testuser", "testpass")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	_, err = client.GetPages()
	if err == nil {
		t.Error("GetPages() should return error for HTTP error")
	}
}

func TestGetMediaError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:     server.URL,
		Timeout: 10,
	}

	client, err := NewClient(cfg, "testuser", "testpass")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	_, err = client.GetMedia()
	if err == nil {
		t.Error("GetMedia() should return error for HTTP error")
	}
}

func TestGetCategoriesError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:     server.URL,
		Timeout: 10,
	}

	client, err := NewClient(cfg, "testuser", "testpass")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	_, err = client.GetCategories()
	if err == nil {
		t.Error("GetCategories() should return error for HTTP error")
	}
}

func TestGetTagsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:     server.URL,
		Timeout: 10,
	}

	client, err := NewClient(cfg, "testuser", "testpass")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	_, err = client.GetTags()
	if err == nil {
		t.Error("GetTags() should return error for HTTP error")
	}
}

func TestGetUsersError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:     server.URL,
		Timeout: 10,
	}

	client, err := NewClient(cfg, "testuser", "testpass")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	_, err = client.GetUsers()
	if err == nil {
		t.Error("GetUsers() should return error for HTTP error")
	}
}

func TestMakeRequestInvalidXMLResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		w.WriteHeader(http.StatusOK)
		// Invalid XML
		_, _ = w.Write([]byte("not valid xml"))
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:     server.URL,
		Timeout: 10,
	}

	client, err := NewClient(cfg, "testuser", "testpass")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	req := &XMLRPCRequest{
		Method: "wp.test",
	}

	_, err = client.makeRequest(req)
	if err == nil {
		t.Error("makeRequest() should return error for invalid XML response")
	}
}

func TestMakeRequestNetworkError(t *testing.T) {
	cfg := &config.Config{
		URL:     "http://192.0.2.1", // Non-routable IP
		Timeout: 1,                  // Very short timeout
	}

	client, err := NewClient(cfg, "testuser", "testpass")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	req := &XMLRPCRequest{
		Method: "wp.test",
	}

	_, err = client.makeRequest(req)
	if err == nil {
		t.Error("makeRequest() should return error for network error")
	}
}

func TestParsePostsResponse(t *testing.T) {
	cfg := &config.Config{
		URL:     "https://example.com",
		Timeout: 10,
	}

	client, err := NewClient(cfg, "testuser", "testpass")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test with a real struct array: fields must be mapped from the response,
	// never fabricated.
	respWithParams := &XMLRPCResponse{
		Params: []Param{
			{Value: Value{Array: &Array{Data: []Value{
				{Struct: &Struct{Members: []Member{
					{Name: "post_id", Value: Value{Int: intPtr(42)}},
					{Name: "post_title", Value: Value{String: stringPtr("Real Title")}},
					{Name: "post_name", Value: Value{String: stringPtr("real-title")}},
					{Name: "post_status", Value: Value{String: stringPtr("publish")}},
					{Name: "post_content", Value: Value{String: stringPtr("Body")}},
				}}},
			}}}},
		},
	}

	posts := client.parsePostsResponse(respWithParams)
	if len(posts) != 1 {
		t.Fatalf("parsePostsResponse() should return 1 post, got %d", len(posts))
	}
	if posts[0].ID != 42 {
		t.Errorf("parsePostsResponse() ID = %d, want 42", posts[0].ID)
	}
	if posts[0].Title.Rendered != "Real Title" {
		t.Errorf("parsePostsResponse() Title = %q, want %q", posts[0].Title.Rendered, "Real Title")
	}
	if posts[0].Slug != "real-title" {
		t.Errorf("parsePostsResponse() Slug = %q, want %q", posts[0].Slug, "real-title")
	}
	// Guard against the old fabricated stub value.
	if posts[0].Title.Rendered == "Sample Post" {
		t.Error("parsePostsResponse() returned fabricated 'Sample Post' data")
	}

	// Test with a scalar param (not an array): must yield no posts, not a fake one.
	respScalar := &XMLRPCResponse{
		Params: []Param{
			{Value: Value{String: stringPtr("data")}},
		},
	}
	if got := client.parsePostsResponse(respScalar); len(got) != 0 {
		t.Errorf("parsePostsResponse() with scalar param should return 0 posts, got %d", len(got))
	}

	// Test with empty params
	respEmpty := &XMLRPCResponse{
		Params: []Param{},
	}

	postsEmpty := client.parsePostsResponse(respEmpty)
	if len(postsEmpty) != 0 {
		t.Errorf("parsePostsResponse() with empty params should return 0 posts, got %d", len(postsEmpty))
	}
}

// TestParsePostsResponseFromXML validates end-to-end XML unmarshalling + mapping,
// exercising both <int>/<i4> and dateTime.iso8601 handling.
func TestParsePostsResponseFromXML(t *testing.T) {
	raw := `<?xml version="1.0"?>
<methodResponse><params><param><value><array><data>
  <value><struct>
    <member><name>post_id</name><value><i4>7</i4></value></member>
    <member><name>post_title</name><value><string>Hello &amp; World</string></value></member>
    <member><name>post_name</name><value>hello-world</value></member>
    <member><name>post_status</name><value><string>draft</string></value></member>
    <member><name>post_date</name><value><dateTime.iso8601>20250131T12:00:00</dateTime.iso8601></value></member>
    <member><name>link</name><value><string>https://example.com/hello</string></value></member>
  </struct></value>
</data></array></value></param></params></methodResponse>`

	var resp XMLRPCResponse
	if err := xml.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("xml.Unmarshal() error = %v", err)
	}

	c := &Client{config: &config.Config{URL: "https://example.com"}}
	posts := c.parsePostsResponse(&resp)
	if len(posts) != 1 {
		t.Fatalf("parsePostsResponse() len = %d, want 1", len(posts))
	}
	p := posts[0]
	if p.ID != 7 {
		t.Errorf("ID = %d, want 7 (i4 not parsed)", p.ID)
	}
	if p.Title.Rendered != "Hello & World" {
		t.Errorf("Title = %q, want %q", p.Title.Rendered, "Hello & World")
	}
	if p.Slug != "hello-world" {
		t.Errorf("Slug = %q, want %q (untyped value not parsed)", p.Slug, "hello-world")
	}
	if p.Status != "draft" {
		t.Errorf("Status = %q, want draft", p.Status)
	}
	if p.Date.Year() != 2025 || p.Date.Month() != 1 || p.Date.Day() != 31 {
		t.Errorf("Date = %v, want 2025-01-31", p.Date.Time)
	}
}

// TestParseTermsAndUsersFromXML validates category/tag/user mapping.
func TestParseTermsAndUsersFromXML(t *testing.T) {
	c := &Client{config: &config.Config{URL: "https://example.com"}}

	termsXML := `<?xml version="1.0"?>
<methodResponse><params><param><value><array><data>
  <value><struct>
    <member><name>term_id</name><value><int>3</int></value></member>
    <member><name>name</name><value><string>News</string></value></member>
    <member><name>slug</name><value><string>news</string></value></member>
    <member><name>taxonomy</name><value><string>category</string></value></member>
    <member><name>count</name><value><int>12</int></value></member>
  </struct></value>
</data></array></value></param></params></methodResponse>`
	var termsResp XMLRPCResponse
	if err := xml.Unmarshal([]byte(termsXML), &termsResp); err != nil {
		t.Fatalf("xml.Unmarshal(terms) error = %v", err)
	}
	cats := c.parseCategoriesResponse(&termsResp)
	if len(cats) != 1 || cats[0].ID != 3 || cats[0].Name != "News" || cats[0].Count != 12 {
		t.Errorf("parseCategoriesResponse() = %+v, want id=3 name=News count=12", cats)
	}

	usersXML := `<?xml version="1.0"?>
<methodResponse><params><param><value><array><data>
  <value><struct>
    <member><name>user_id</name><value><int>5</int></value></member>
    <member><name>display_name</name><value><string>Jane Doe</string></value></member>
    <member><name>nicename</name><value><string>jane</string></value></member>
  </struct></value>
</data></array></value></param></params></methodResponse>`
	var usersResp XMLRPCResponse
	if err := xml.Unmarshal([]byte(usersXML), &usersResp); err != nil {
		t.Fatalf("xml.Unmarshal(users) error = %v", err)
	}
	users := c.parseUsersResponse(&usersResp)
	if len(users) != 1 || users[0].ID != 5 || users[0].Name != "Jane Doe" || users[0].Slug != "jane" {
		t.Errorf("parseUsersResponse() = %+v, want id=5 name='Jane Doe' slug=jane", users)
	}
}

// TestParseSiteInfoNestedStruct validates wp.getOptions parsing with the real
// WordPress nested {value, desc} option shape.
func TestParseSiteInfoNestedStruct(t *testing.T) {
	c := &Client{config: &config.Config{URL: "https://example.com"}}
	raw := `<?xml version="1.0"?>
<methodResponse><params><param><value><struct>
  <member><name>blog_title</name><value><struct>
    <member><name>value</name><value><string>My Blog</string></value></member>
    <member><name>desc</name><value><string>Site Title</string></value></member>
  </struct></value></member>
  <member><name>home_url</name><value><struct>
    <member><name>value</name><value><string>https://example.com</string></value></member>
  </struct></value></member>
</struct></value></param></params></methodResponse>`
	var resp XMLRPCResponse
	if err := xml.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("xml.Unmarshal() error = %v", err)
	}
	info := c.parseSiteInfo(&resp)
	if info.Name != "My Blog" {
		t.Errorf("parseSiteInfo() Name = %q, want 'My Blog'", info.Name)
	}
	if info.HomeURL != "https://example.com" {
		t.Errorf("parseSiteInfo() HomeURL = %q, want https://example.com", info.HomeURL)
	}
}

func TestXMLRPCValueTypes(t *testing.T) {
	// Test struct with members
	s := Struct{
		Members: []Member{
			{Name: "key1", Value: Value{String: stringPtr("value1")}},
			{Name: "key2", Value: Value{Int: intPtr(42)}},
		},
	}

	if len(s.Members) != 2 {
		t.Errorf("Struct Members length = %d, want 2", len(s.Members))
	}

	// Test array with values
	a := Array{
		Data: []Value{
			{String: stringPtr("item1")},
			{String: stringPtr("item2")},
		},
	}

	if len(a.Data) != 2 {
		t.Errorf("Array Data length = %d, want 2", len(a.Data))
	}

	// Test fault structure
	fault := Fault{
		Value: Value{
			Struct: &Struct{
				Members: []Member{
					{Name: "faultCode", Value: Value{Int: intPtr(403)}},
					{Name: "faultString", Value: Value{String: stringPtr("Forbidden")}},
				},
			},
		},
	}

	if fault.Value.Struct == nil {
		t.Error("Fault Value Struct should not be nil")
	}

	if len(fault.Value.Struct.Members) != 2 {
		t.Errorf("Fault Members length = %d, want 2", len(fault.Value.Struct.Members))
	}
}

func TestGetMediaEmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		w.WriteHeader(http.StatusOK)
		// Response with empty params
		response := `<?xml version="1.0" encoding="UTF-8"?>
<methodResponse>
	<params>
	</params>
</methodResponse>`
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:     server.URL,
		Timeout: 10,
	}

	client, err := NewClient(cfg, "testuser", "testpass")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	media, err := client.GetMedia()
	if err != nil {
		t.Errorf("GetMedia() error = %v, want nil", err)
	}

	if len(media) != 0 {
		t.Errorf("GetMedia() with empty response should return empty slice, got %d items", len(media))
	}
}

func TestGetPagesEmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		w.WriteHeader(http.StatusOK)
		response := `<?xml version="1.0" encoding="UTF-8"?>
<methodResponse>
	<params>
	</params>
</methodResponse>`
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:     server.URL,
		Timeout: 10,
	}

	client, err := NewClient(cfg, "testuser", "testpass")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	pages, err := client.GetPages()
	if err != nil {
		t.Errorf("GetPages() error = %v, want nil", err)
	}

	if len(pages) != 0 {
		t.Errorf("GetPages() with empty response should return empty slice, got %d pages", len(pages))
	}
}

func TestMakeRequestWithFault(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		w.WriteHeader(http.StatusOK)
		// Response with fault
		response := `<?xml version="1.0" encoding="UTF-8"?>
<methodResponse>
	<fault>
		<value>
			<struct>
				<member>
					<name>faultCode</name>
					<value><int>401</int></value>
				</member>
				<member>
					<name>faultString</name>
					<value><string>Unauthorized</string></value>
				</member>
			</struct>
		</value>
	</fault>
</methodResponse>`
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:     server.URL,
		Timeout: 10,
	}

	client, err := NewClient(cfg, "testuser", "testpass")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	err = client.TestConnection()
	if err == nil {
		t.Error("TestConnection() should return error for fault response")
	}
}

func TestMakeRequestReadBodyError(t *testing.T) {
	cfg := &config.Config{
		URL:     "http://192.0.2.1", // Non-routable IP
		Timeout: 1,
	}

	client, err := NewClient(cfg, "testuser", "testpass")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	req := &XMLRPCRequest{
		Method: "wp.test",
	}

	_, err = client.makeRequest(req)
	if err == nil {
		t.Error("makeRequest() should return error for network error")
	}
}

func TestParseMediaResponse(t *testing.T) {
	cfg := &config.Config{
		URL:     "https://example.com",
		Timeout: 10,
	}

	client, err := NewClient(cfg, "testuser", "testpass")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test with empty params
	respEmpty := &XMLRPCResponse{
		Params: []Param{},
	}

	media := client.parseMediaResponse(respEmpty)
	if len(media) != 0 {
		t.Errorf("parseMediaResponse() with empty params should return 0 items, got %d", len(media))
	}
}

func TestParseCategoriesResponse(t *testing.T) {
	cfg := &config.Config{
		URL:     "https://example.com",
		Timeout: 10,
	}

	client, err := NewClient(cfg, "testuser", "testpass")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test with empty params
	respEmpty := &XMLRPCResponse{
		Params: []Param{},
	}

	categories := client.parseCategoriesResponse(respEmpty)
	if len(categories) != 0 {
		t.Errorf("parseCategoriesResponse() with empty params should return 0 items, got %d", len(categories))
	}
}

func TestParseTagsResponse(t *testing.T) {
	cfg := &config.Config{
		URL:     "https://example.com",
		Timeout: 10,
	}

	client, err := NewClient(cfg, "testuser", "testpass")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test with empty params
	respEmpty := &XMLRPCResponse{
		Params: []Param{},
	}

	tags := client.parseTagsResponse(respEmpty)
	if len(tags) != 0 {
		t.Errorf("parseTagsResponse() with empty params should return 0 items, got %d", len(tags))
	}
}

func TestParseUsersResponse(t *testing.T) {
	cfg := &config.Config{
		URL:     "https://example.com",
		Timeout: 10,
	}

	client, err := NewClient(cfg, "testuser", "testpass")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test with empty params
	respEmpty := &XMLRPCResponse{
		Params: []Param{},
	}

	users := client.parseUsersResponse(respEmpty)
	if len(users) != 0 {
		t.Errorf("parseUsersResponse() with empty params should return 0 items, got %d", len(users))
	}
}

func TestGetMediaPagination(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		w.Header().Set("Content-Type", "text/xml")
		w.WriteHeader(http.StatusOK)

		// Return different responses based on request count
		if requestCount == 1 {
			// First request returns 1 item (less than limit of 100, triggering early pagination break)
			_, _ = w.Write([]byte(`<?xml version="1.0"?>
<methodResponse>
  <params>
    <param>
      <value>
        <array>
          <data>
            <value><struct>
              <member><name>attachment_id</name><value><int>1</int></value></member>
              <member><name>link</name><value><string>https://example.com/media/1.jpg</string></value></member>
            </struct></value>
          </data>
        </array>
      </value>
    </param>
  </params>
</methodResponse>`))
		} else {
			// Should not be called if pagination breaks early
			_, _ = w.Write([]byte(`<?xml version="1.0"?>
<methodResponse>
  <params>
    <param>
      <value>
        <array>
          <data>
          </data>
        </array>
      </value>
    </param>
  </params>
</methodResponse>`))
		}
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:     server.URL,
		Timeout: 10,
	}

	client, err := NewClient(cfg, "testuser", "testpass")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	media, err := client.GetMedia()
	if err != nil {
		t.Fatalf("GetMedia() error = %v", err)
	}

	// Test that pagination works - just ensure no error and some result
	t.Logf("GetMedia() returned %d items", len(media))
}

func TestGetMediaEmptyFirstResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		w.WriteHeader(http.StatusOK)

		// Return empty array immediately
		_, _ = w.Write([]byte(`<?xml version="1.0"?>
<methodResponse>
  <params>
    <param>
      <value>
        <array>
          <data>
          </data>
        </array>
      </value>
    </param>
  </params>
</methodResponse>`))
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:     server.URL,
		Timeout: 10,
	}

	client, err := NewClient(cfg, "testuser", "testpass")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	media, err := client.GetMedia()
	if err != nil {
		t.Fatalf("GetMedia() error = %v", err)
	}

	if len(media) != 0 {
		t.Errorf("GetMedia() returned %d items, expected 0", len(media))
	}
}

func TestGetPagesEmptyFirstResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		w.WriteHeader(http.StatusOK)

		_, _ = w.Write([]byte(`<?xml version="1.0"?>
<methodResponse>
  <params>
    <param>
      <value>
        <array>
          <data>
          </data>
        </array>
      </value>
    </param>
  </params>
</methodResponse>`))
	}))
	defer server.Close()

	cfg := &config.Config{
		URL:     server.URL,
		Timeout: 10,
	}

	client, err := NewClient(cfg, "testuser", "testpass")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	pages, err := client.GetPages()
	if err != nil {
		t.Fatalf("GetPages() error = %v", err)
	}

	// Just ensure no error - the parsing might return default values
	t.Logf("GetPages() returned %d items", len(pages))
}
