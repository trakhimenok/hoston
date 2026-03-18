package github

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/trakhimenok/hoston/internal/provider"
)

// Compile-time check that *Provider implements provider.HostingProvider.
var _ provider.HostingProvider = (*Provider)(nil)

// Provider implements provider.HostingProvider for GitHub Pages.
type Provider struct{}

// NewProvider returns a new GitHub Pages Provider.
func NewProvider() *Provider {
	return &Provider{}
}

// Name returns a human-readable name for the provider.
func (p *Provider) Name() string {
	return "GitHub Pages"
}

// Setup enables GitHub Pages for the given repository, sets the custom domain,
// and returns the DNS records required to point the domain at GitHub Pages.
//
// Required params:
//
//	"repo" — repository in "owner/repo" format.
func (p *Provider) Setup(ctx context.Context, domain string, params map[string]string) ([]provider.DNSRecord, error) {
	repo, ok := params["repo"]
	if !ok || repo == "" {
		return nil, fmt.Errorf("github provider: missing required param \"repo\" (expected \"owner/repo\" format)")
	}

	if err := EnablePages(repo); err != nil {
		return nil, fmt.Errorf("github provider: %w", err)
	}

	if err := SetCustomDomain(repo, domain); err != nil {
		return nil, fmt.Errorf("github provider: %w", err)
	}

	return GetRequiredDNSRecords(domain), nil
}

// PagesConfig holds GitHub Pages configuration.
type PagesConfig struct {
	Source string
	Branch string
}

// EnablePages enables GitHub Pages for a repository.
func EnablePages(repo string) error {
	cmd := exec.Command("gh", "api",
		fmt.Sprintf("repos/%s/pages", repo),
		"-X", "POST",
		"-f", "build_type=workflow",
		"-f", "source[branch]=main",
		"-f", "source[path]=/",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to enable GitHub Pages for %s: %s", repo, string(out))
	}
	fmt.Printf("✓ GitHub Pages enabled for %s\n", repo)
	return nil
}

// SetCustomDomain sets a custom domain for GitHub Pages.
func SetCustomDomain(repo, domain string) error {
	cmd := exec.Command("gh", "api",
		fmt.Sprintf("repos/%s/pages", repo),
		"-X", "PUT",
		"-f", fmt.Sprintf("cname=%s", domain),
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to set custom domain for %s: %s", repo, string(out))
	}
	fmt.Printf("✓ Custom domain set to %s for %s\n", domain, repo)
	return nil
}

// GetPagesInfo returns current Pages configuration.
func GetPagesInfo(repo string) (*PagesInfo, error) {
	cmd := exec.Command("gh", "api", fmt.Sprintf("repos/%s/pages", repo))
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("GitHub Pages not configured for %s", repo)
	}

	var info PagesInfo
	if err := json.Unmarshal(out, &info); err != nil {
		return nil, fmt.Errorf("failed to parse Pages info: %w", err)
	}
	return &info, nil
}

// PagesInfo represents GitHub Pages status.
type PagesInfo struct {
	URL    string `json:"html_url"`
	Status string `json:"status"`
	CNAME  string `json:"cname"`
}

// GetRequiredDNSRecords returns DNS records needed for GitHub Pages.
func GetRequiredDNSRecords(domain string) []provider.DNSRecord {
	// GitHub Pages uses these A records for apex domains.
	return []provider.DNSRecord{
		{Type: "A", Name: domain, Content: "185.199.108.153"},
		{Type: "A", Name: domain, Content: "185.199.109.153"},
		{Type: "A", Name: domain, Content: "185.199.110.153"},
		{Type: "A", Name: domain, Content: "185.199.111.153"},
	}
}
