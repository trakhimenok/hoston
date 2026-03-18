package firebase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/trakhimenok/hoston/internal/provider"
)

// ErrSiteAlreadyExists is returned by Setup when the Firebase Hosting site
// already exists and the caller has not set params["use_existing"] = "true".
// The caller (typically the wizard) should catch this, confirm with the user,
// and retry with that param set.
var ErrSiteAlreadyExists = errors.New("firebase: site already exists")

// Compile-time check that *Provider implements provider.HostingProvider.
var _ provider.HostingProvider = (*Provider)(nil)

// Provider implements provider.HostingProvider for Firebase Hosting.
type Provider struct{}

// NewProvider returns a new Firebase Hosting Provider.
func NewProvider() *Provider {
	return &Provider{}
}

// Name returns the human-readable name of this hosting provider.
func (p *Provider) Name() string {
	return "Firebase Hosting"
}

// Setup creates (or verifies) a Firebase Hosting site for the given domain and
// returns the DNS records that must be configured at the DNS provider.
//
// Required params:
//   - "project_id"    – Firebase project ID
//   - "site_name"     – Firebase Hosting site name (subdomain of web.app)
//   - "use_existing"  – (optional) "true" to proceed with an existing site
func (p *Provider) Setup(ctx context.Context, domain string, params map[string]string) ([]provider.DNSRecord, error) {
	projectID := params["project_id"]
	siteName := params["site_name"]

	if projectID == "" {
		return nil, fmt.Errorf("firebase: missing required param \"project_id\"")
	}
	if siteName == "" {
		return nil, fmt.Errorf("firebase: missing required param \"site_name\"")
	}

	exists, err := SiteExists(projectID, siteName)
	if err != nil {
		return nil, fmt.Errorf("firebase: checking site existence: %w", err)
	}

	if exists {
		if params["use_existing"] != "true" {
			return nil, ErrSiteAlreadyExists
		}
		// User confirmed — proceed with the existing site.
	} else {
		if err := CreateSite(projectID, siteName); err != nil {
			return nil, fmt.Errorf("firebase: creating site: %w", err)
		}
	}

	localRecords, err := AddCustomDomain(projectID, siteName, domain)
	if err != nil {
		return nil, fmt.Errorf("firebase: adding custom domain: %w", err)
	}

	// Fall back to well-known static records when the CLI returns none.
	if len(localRecords) == 0 {
		localRecords = GetRequiredDNSRecords(domain)
	}

	return localRecords, nil
}

// SiteExists checks if a Firebase Hosting site exists.
func SiteExists(projectID, siteName string) (bool, error) {
	// "firebase hosting:sites:get" targets a single site — faster and simpler
	// than listing all sites and searching.
	cmd := exec.Command("firebase", "hosting:sites:get", siteName, "--project", projectID)
	if err := cmd.Run(); err == nil {
		return true, nil
	}

	// Fallback: list all sites (works with older Firebase CLI versions).
	cmd = exec.Command("firebase", "hosting:sites:list", "--project", projectID, "--json")
	out, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to list hosting sites: %w", err)
	}

	// The Firebase CLI --json output may use a {"status":"success","result":{…}}
	// envelope, or return the payload directly. Try both shapes.
	type siteEntry struct {
		Name string `json:"name"`
	}

	fullName := fmt.Sprintf("projects/%s/sites/%s", projectID, siteName)
	matchesSite := func(name string) bool {
		return name == fullName || name == siteName
	}

	// Shape 1: envelope with result.sites
	var envelope struct {
		Result struct {
			Sites []siteEntry `json:"sites"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out, &envelope); err == nil && len(envelope.Result.Sites) > 0 {
		for _, s := range envelope.Result.Sites {
			if matchesSite(s.Name) {
				return true, nil
			}
		}
		return false, nil
	}

	// Shape 2: flat {"sites":[…]}
	var flat struct {
		Sites []siteEntry `json:"sites"`
	}
	if err := json.Unmarshal(out, &flat); err != nil {
		return false, fmt.Errorf("failed to parse sites response: %w", err)
	}
	for _, s := range flat.Sites {
		if matchesSite(s.Name) {
			return true, nil
		}
	}
	return false, nil
}

// CreateSite creates a Firebase Hosting site.
// Returns nil if the site already exists (409).
func CreateSite(projectID, siteName string) error {
	cmd := exec.Command("firebase", "hosting:sites:create", siteName, "--project", projectID)
	out, err := cmd.CombinedOutput()
	if err != nil {
		output := string(out)
		if strings.Contains(output, "409") || strings.Contains(strings.ToLower(output), "already exists") {
			return nil
		}
		return fmt.Errorf("failed to create hosting site %s: %s", siteName, output)
	}
	return nil
}

// DeployPlaceholder deploys a minimal placeholder page.
func DeployPlaceholder(projectID, siteName string) error {
	// Firebase deploy requires a firebase.json and public directory.
	// We'll use the CLI's --only hosting approach with a temp config.
	cmd := exec.Command("firebase", "hosting:channel:deploy", "live",
		"--project", projectID,
		"--site", siteName,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to deploy placeholder: %s", string(out))
	}
	fmt.Printf("✓ Deployed to %s.web.app\n", siteName)
	return nil
}

// GetAutoURL returns the auto-generated hosting URL.
func GetAutoURL(siteName string) string {
	return fmt.Sprintf("https://%s.web.app", siteName)
}

// AddCustomDomain adds a custom domain to Firebase Hosting.
// Returns required DNS records (TXT for verification, A/AAAA for pointing).
func AddCustomDomain(projectID, siteName, domain string) ([]provider.DNSRecord, error) {
	cmd := exec.Command("firebase", "hosting:custom-domains:create",
		domain,
		"--project", projectID,
		"--site", siteName,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		output := string(out)
		// Try to extract DNS records from error/output.
		records := parseDNSRecords(output)
		if len(records) > 0 {
			return records, nil
		}
		return nil, fmt.Errorf("failed to add custom domain: %s", output)
	}

	records := parseDNSRecords(string(out))
	return records, nil
}

// GetRequiredDNSRecords returns the standard Firebase Hosting DNS records for apex domains.
func GetRequiredDNSRecords(domain string) []provider.DNSRecord {
	return []provider.DNSRecord{
		{Type: "A", Name: domain, Content: "199.36.158.100"},
		{Type: "TXT", Name: fmt.Sprintf("_acme-challenge.%s", domain), Content: "(auto-generated by Firebase)"},
	}
}

func parseDNSRecords(output string) []provider.DNSRecord {
	var records []provider.DNSRecord
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Look for lines that contain record info (heuristic).
		if strings.Contains(line, "TXT") || strings.Contains(line, " A ") || strings.Contains(line, "AAAA") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				records = append(records, provider.DNSRecord{
					Type:    parts[0],
					Name:    parts[1],
					Content: parts[2],
				})
			}
		}
	}
	return records
}
