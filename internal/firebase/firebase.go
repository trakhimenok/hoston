package firebase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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

	// The Firebase CLI does not support adding custom domains — it must be
	// done via the Firebase Console.  Return the well-known DNS records so
	// hoston can configure them at the DNS provider, and surface a console
	// link for the manual step.
	return GetRequiredDNSRecords(domain, siteName), nil
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

// DeployPlaceholder creates a temporary directory with a "Coming Soon" page
// and deploys it to the Firebase Hosting site via `firebase deploy`.
func DeployPlaceholder(projectID, siteName, domain string) error {
	dir, err := os.MkdirTemp("", "hoston-deploy-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	// firebase.json
	fbJSON := fmt.Sprintf(`{
  "hosting": {
    "site": %q,
    "public": "public",
    "ignore": ["firebase.json", "**/node_modules/**"]
  }
}`, siteName)
	if err := os.WriteFile(filepath.Join(dir, "firebase.json"), []byte(fbJSON), 0644); err != nil {
		return err
	}

	// public/index.html
	pubDir := filepath.Join(dir, "public")
	if err := os.MkdirAll(pubDir, 0755); err != nil {
		return err
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>%s</title>
  <style>
    *{margin:0;padding:0;box-sizing:border-box}
    body{min-height:100vh;display:flex;align-items:center;justify-content:center;
         font-family:system-ui,-apple-system,sans-serif;background:#0f172a;color:#e2e8f0}
    .card{text-align:center;padding:3rem}
    h1{font-size:2.5rem;font-weight:700;margin-bottom:.75rem;
       background:linear-gradient(135deg,#38bdf8,#818cf8);-webkit-background-clip:text;
       -webkit-text-fill-color:transparent}
    p{font-size:1.25rem;color:#94a3b8}
  </style>
</head>
<body>
  <div class="card">
    <h1>%s</h1>
    <p>Coming Soon</p>
  </div>
</body>
</html>`, domain, domain)

	if err := os.WriteFile(filepath.Join(pubDir, "index.html"), []byte(html), 0644); err != nil {
		return err
	}

	cmd := exec.Command("firebase", "deploy", "--only", "hosting:"+siteName, "--project", projectID)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("deploy failed: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// ConsoleURL returns the Firebase Console URL for adding a custom domain.
func ConsoleURL(projectID, siteName string) string {
	return fmt.Sprintf("https://console.firebase.google.com/project/%s/hosting/sites/%s", projectID, siteName)
}

// GetAutoURL returns the auto-generated hosting URL.
func GetAutoURL(siteName string) string {
	return fmt.Sprintf("https://%s.web.app", siteName)
}

// GetRequiredDNSRecords returns the DNS records needed for Firebase Hosting:
//   - A TXT ownership-verification record (hosting-site=<site>)
//   - An A record pointing to Firebase Hosting
func GetRequiredDNSRecords(domain, siteName string) []provider.DNSRecord {
	return []provider.DNSRecord{
		{Type: "TXT", Name: domain, Content: fmt.Sprintf("\"hosting-site=%s\"", siteName)},
		{Type: "A", Name: domain, Content: "199.36.158.100"},
	}
}
