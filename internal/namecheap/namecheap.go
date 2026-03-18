package namecheap

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	sandboxURL    = "https://api.sandbox.namecheap.com/xml.response"
	productionURL = "https://api.namecheap.com/xml.response"
)

// Client wraps the NameCheap API.
type Client struct {
	apiUser    string
	apiKey     string
	username   string
	clientIP   string
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a NameCheap API client.
func NewClient(apiUser, apiKey, username, clientIP string, sandbox bool) *Client {
	baseURL := productionURL
	if sandbox {
		baseURL = sandboxURL
	}
	return &Client{
		apiUser:    apiUser,
		apiKey:     apiKey,
		username:   username,
		clientIP:   clientIP,
		baseURL:    baseURL,
		httpClient: &http.Client{},
	}
}

// apiResponse is the top-level NameCheap API response.
type apiResponse struct {
	XMLName xml.Name `xml:"ApiResponse"`
	Status  string   `xml:"Status,attr"`
	Errors  struct {
		Error []struct {
			Number  string `xml:"Number,attr"`
			Message string `xml:",chardata"`
		} `xml:"Error"`
	} `xml:"Errors"`
	CommandResponse struct {
		Inner []byte `xml:",innerxml"`
	} `xml:"CommandResponse"`
}

// SetCustomNameservers sets custom nameservers for a domain.
func (c *Client) SetCustomNameservers(domain string, nameservers []string) error {
	parts := strings.SplitN(domain, ".", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid domain format: %s (expected name.tld)", domain)
	}
	sld, tld := parts[0], parts[1]

	params := url.Values{
		"ApiUser":     {c.apiUser},
		"ApiKey":      {c.apiKey},
		"UserName":    {c.username},
		"ClientIp":    {c.clientIP},
		"Command":     {"namecheap.domains.dns.setCustom"},
		"SLD":         {sld},
		"TLD":         {tld},
		"Nameservers": {strings.Join(nameservers, ",")},
	}

	resp, err := c.httpClient.Get(c.baseURL + "?" + params.Encode())
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	var apiResp apiResponse
	if err := xml.Unmarshal(body, &apiResp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if apiResp.Status != "OK" {
		var errMsgs []string
		for _, e := range apiResp.Errors.Error {
			errMsgs = append(errMsgs, e.Message)
		}
		return fmt.Errorf("NameCheap API error: %s", strings.Join(errMsgs, "; "))
	}

	return nil
}

// GetNameservers retrieves the current nameservers for a domain.
func (c *Client) GetNameservers(domain string) ([]string, error) {
	parts := strings.SplitN(domain, ".", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid domain format: %s", domain)
	}
	sld, tld := parts[0], parts[1]

	params := url.Values{
		"ApiUser":  {c.apiUser},
		"ApiKey":   {c.apiKey},
		"UserName": {c.username},
		"ClientIp": {c.clientIP},
		"Command":  {"namecheap.domains.dns.getList"},
		"SLD":      {sld},
		"TLD":      {tld},
	}

	resp, err := c.httpClient.Get(c.baseURL + "?" + params.Encode())
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var apiResp apiResponse
	if err := xml.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if apiResp.Status != "OK" {
		var errMsgs []string
		for _, e := range apiResp.Errors.Error {
			errMsgs = append(errMsgs, e.Message)
		}
		return nil, fmt.Errorf("NameCheap API error: %s", strings.Join(errMsgs, "; "))
	}

	// Parse nameservers from inner XML
	type nsResult struct {
		XMLName     xml.Name `xml:"DomainDNSGetListResult"`
		Nameservers []string `xml:"Nameserver"`
	}
	var result nsResult
	if err := xml.Unmarshal(apiResp.CommandResponse.Inner, &result); err != nil {
		return nil, fmt.Errorf("failed to parse nameservers: %w", err)
	}

	return result.Nameservers, nil
}
