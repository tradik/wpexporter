package xmlrpc

import (
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
	}

	if siteInfo.URL != server.URL {
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
	}

	if len(resp.Params) == 0 {
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
