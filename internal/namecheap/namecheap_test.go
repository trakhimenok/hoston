package namecheap

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// successSetCustomXML is a minimal NameCheap OK response for setCustom.
const successSetCustomXML = `<?xml version="1.0" encoding="utf-8"?>
<ApiResponse Status="OK">
  <Errors/>
  <CommandResponse/>
</ApiResponse>`

// errorSetCustomXML is a NameCheap ERROR response.
const errorSetCustomXML = `<?xml version="1.0" encoding="utf-8"?>
<ApiResponse Status="ERROR">
  <Errors>
    <Error Number="2019166">Parameter Nameservers is Missing</Error>
  </Errors>
  <CommandResponse/>
</ApiResponse>`

// successGetListXML is a NameCheap OK response carrying two nameservers.
const successGetListXML = `<?xml version="1.0" encoding="utf-8"?>
<ApiResponse Status="OK">
  <Errors/>
  <CommandResponse>
    <DomainDNSGetListResult>
      <Nameserver>ns1.example.com.</Nameserver>
      <Nameserver>ns2.example.com.</Nameserver>
    </DomainDNSGetListResult>
  </CommandResponse>
</ApiResponse>`

// newTestClient returns a Client whose baseURL points to the given test server.
func newTestClient(server *httptest.Server) *Client {
	return &Client{
		apiUser:    "testUser",
		apiKey:     "testKey",
		username:   "testUsername",
		clientIP:   "1.2.3.4",
		baseURL:    server.URL,
		httpClient: server.Client(),
	}
}

// TestSetCustomNameservers_Success verifies that a 200 OK XML response causes
// SetCustomNameservers to return nil.
func TestSetCustomNameservers_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		fmt.Fprint(w, successSetCustomXML)
	}))
	defer server.Close()

	client := newTestClient(server)
	err := client.SetCustomNameservers("example.com", []string{"ns1.cloudflare.com", "ns2.cloudflare.com"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

// TestSetCustomNameservers_APIError verifies that a NameCheap error response is
// surfaced as a non-nil error containing the error message.
func TestSetCustomNameservers_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		fmt.Fprint(w, errorSetCustomXML)
	}))
	defer server.Close()

	client := newTestClient(server)
	err := client.SetCustomNameservers("example.com", []string{"ns1.cloudflare.com"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	const want = "NameCheap API error"
	if got := err.Error(); len(got) < len(want) || got[:len(want)] != want {
		t.Errorf("error message should start with %q, got %q", want, got)
	}
}

// TestSetCustomNameservers_ServerError verifies that an HTTP-level failure
// (server closes connection) produces a non-nil error.
func TestSetCustomNameservers_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Respond with an HTTP 500 and invalid XML so both layers can be exercised.
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "internal server error")
	}))
	defer server.Close()

	client := newTestClient(server)
	err := client.SetCustomNameservers("example.com", []string{"ns1.cloudflare.com"})
	// The XML unmarshal will fail, so we expect an error.
	if err == nil {
		t.Fatal("expected error for invalid XML response, got nil")
	}
}

// TestSetCustomNameservers_InvalidDomain verifies that a malformed domain
// (missing TLD separator) is rejected before any HTTP call is made.
func TestSetCustomNameservers_InvalidDomain(t *testing.T) {
	// Server should never be called; if it is, the test will still pass but the
	// server handler helps us detect the unexpected request via the error return.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("HTTP request should not have been made for an invalid domain")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := newTestClient(server)
	err := client.SetCustomNameservers("nodot", []string{"ns1.cloudflare.com"})
	if err == nil {
		t.Fatal("expected error for invalid domain, got nil")
	}
}

// TestGetNameservers_Success verifies that a successful response is parsed into
// the expected slice of nameserver strings.
func TestGetNameservers_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		fmt.Fprint(w, successGetListXML)
	}))
	defer server.Close()

	client := newTestClient(server)
	ns, err := client.GetNameservers("example.com")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	want := []string{"ns1.example.com.", "ns2.example.com."}
	if len(ns) != len(want) {
		t.Fatalf("expected %d nameservers, got %d: %v", len(want), len(ns), ns)
	}
	for i, w := range want {
		if ns[i] != w {
			t.Errorf("nameserver[%d]: want %q, got %q", i, w, ns[i])
		}
	}
}

// TestGetNameservers_APIError verifies that an error status in the XML response
// causes GetNameservers to return a non-nil error.
func TestGetNameservers_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		fmt.Fprint(w, errorSetCustomXML)
	}))
	defer server.Close()

	client := newTestClient(server)
	_, err := client.GetNameservers("example.com")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// TestGetNameservers_InvalidDomain verifies domain validation before any HTTP call.
func TestGetNameservers_InvalidDomain(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("HTTP request should not have been made for an invalid domain")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := newTestClient(server)
	_, err := client.GetNameservers("nodot")
	if err == nil {
		t.Fatal("expected error for invalid domain, got nil")
	}
}

// TestNewClient_Sandbox verifies the sandbox base URL is selected when requested.
func TestNewClient_Sandbox(t *testing.T) {
	c := NewClient("u", "k", "u", "1.2.3.4", true)
	if c.baseURL != sandboxURL {
		t.Errorf("sandbox=true: want baseURL %q, got %q", sandboxURL, c.baseURL)
	}
}

// TestNewClient_Production verifies the production base URL is selected by default.
func TestNewClient_Production(t *testing.T) {
	c := NewClient("u", "k", "u", "1.2.3.4", false)
	if c.baseURL != productionURL {
		t.Errorf("sandbox=false: want baseURL %q, got %q", productionURL, c.baseURL)
	}
}

// TestQueryParametersPresent verifies that mandatory query parameters are sent.
func TestQueryParametersPresent(t *testing.T) {
	var capturedQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "text/xml")
		fmt.Fprint(w, successSetCustomXML)
	}))
	defer server.Close()

	client := newTestClient(server)
	_ = client.SetCustomNameservers("example.com", []string{"ns1.test.com"})

	mustContain := []string{"ApiUser=testUser", "ApiKey=testKey", "SLD=example", "TLD=com"}
	for _, param := range mustContain {
		found := false
		// Simple substring search within raw query.
		for i := 0; i+len(param) <= len(capturedQuery); i++ {
			if capturedQuery[i:i+len(param)] == param {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected query to contain %q, got: %s", param, capturedQuery)
		}
	}
}
