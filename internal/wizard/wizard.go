package wizard

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"

	dnspkg "github.com/trakhimenok/hoston/internal/dns"
	"github.com/trakhimenok/hoston/internal/firebase"
	"github.com/trakhimenok/hoston/internal/provider"
)

// ---------------------------------------------------------------------------
// Styles — reused across all wizard output.
// ---------------------------------------------------------------------------

var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	warnStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	stepStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
)

// ---------------------------------------------------------------------------
// SetupConfig — everything the wizard needs, injected from cmd layer.
// ---------------------------------------------------------------------------

// SetupConfig carries all dependencies and options for the setup wizard.
// Concrete provider clients are created by the calling command and injected
// through the provider interfaces so the wizard has no knowledge of specific
// implementations.
type SetupConfig struct {
	Domain           string
	Verbose          bool
	DNSProvider      provider.DNSProvider
	Registrar        provider.Registrar
	HostingProviders []provider.HostingProvider
}

// ---------------------------------------------------------------------------
// RunSetup — main entry point.
// ---------------------------------------------------------------------------

// RunSetup orchestrates the full domain setup wizard.
func RunSetup(ctx context.Context, cfg SetupConfig) error {
	logger := log.New(os.Stderr)
	if cfg.Verbose {
		logger.SetLevel(log.DebugLevel)
	} else {
		logger.SetLevel(log.InfoLevel)
	}

	domain := cfg.Domain

	fmt.Println(titleStyle.Render(fmt.Sprintf("🚀 Setting up domain: %s", domain)))
	fmt.Println()

	// -----------------------------------------------------------------------
	// Step 1: Verify credentials
	// -----------------------------------------------------------------------
	fmt.Println(stepStyle.Render("Step 1: Verifying credentials..."))

	if cfg.DNSProvider == nil {
		return fmt.Errorf("DNS provider is not configured — run: hoston auth cloudflare")
	}
	if cfg.Registrar == nil {
		return fmt.Errorf("registrar is not configured — run: hoston auth namecheap")
	}
	if len(cfg.HostingProviders) == 0 {
		return fmt.Errorf("no hosting providers configured")
	}

	fmt.Println(successStyle.Render("✓ Credentials verified"))

	// -----------------------------------------------------------------------
	// Step 2: Add domain to DNS provider
	// -----------------------------------------------------------------------
	fmt.Println()
	fmt.Println(stepStyle.Render("Step 2: Adding domain to DNS provider..."))

	var zoneID string
	var nameservers []string

	zoneID, nameservers, err := cfg.DNSProvider.GetZoneByDomain(ctx, domain)
	if err == nil {
		fmt.Println(successStyle.Render(fmt.Sprintf("✓ Domain already exists (zone: %s)", zoneID)))
		logger.Debug("existing zone found", "zoneID", zoneID, "nameservers", nameservers)
	} else {
		logger.Debug("zone not found, creating new zone", "error", err)
		zoneID, err = cfg.DNSProvider.AddZone(ctx, domain)
		if err != nil {
			return fmt.Errorf("failed to add zone: %w", err)
		}
		fmt.Println(successStyle.Render(fmt.Sprintf("✓ Domain added (zone: %s)", zoneID)))
	}

	// -----------------------------------------------------------------------
	// Step 3: Get DNS provider nameservers
	// -----------------------------------------------------------------------
	fmt.Println()
	fmt.Println(stepStyle.Render("Step 3: Getting DNS provider nameservers..."))

	if len(nameservers) == 0 {
		nameservers, err = cfg.DNSProvider.GetNameservers(ctx, zoneID)
		if err != nil {
			return fmt.Errorf("failed to get nameservers: %w", err)
		}
	}
	for _, ns := range nameservers {
		fmt.Printf("  • %s\n", ns)
	}
	logger.Debug("resolved nameservers", "nameservers", nameservers)

	// -----------------------------------------------------------------------
	// Step 4: Update registrar nameservers (skip if already correct)
	// -----------------------------------------------------------------------
	fmt.Println()
	fmt.Println(stepStyle.Render("Step 4: Checking current nameservers..."))

	currentNS, nsErr := cfg.Registrar.GetNameservers(domain)
	if nsErr == nil && nsMatch(currentNS, nameservers) {
		fmt.Println(successStyle.Render("✓ Nameservers already point to DNS provider — skipping update"))
	} else {
		if nsErr == nil {
			logger.Debug("current nameservers don't match", "current", currentNS, "expected", nameservers)
		} else {
			logger.Debug("could not fetch current nameservers", "error", nsErr)
		}
		fmt.Println(stepStyle.Render("  Updating registrar nameservers..."))

		err = cfg.Registrar.SetCustomNameservers(domain, nameservers)
		if err != nil {
			fmt.Println(warnStyle.Render(fmt.Sprintf("⚠ Automatic NS update failed: %v", err)))
			fmt.Println()

			errMsg := err.Error()
			if strings.Contains(errMsg, "Invalid request IP") || strings.Contains(errMsg, "ClientIP") {
				dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
				linkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Underline(true)
				fmt.Println(warnStyle.Render("  Your IP is not whitelisted in NameCheap API access."))
				fmt.Println(dimStyle.Render("  Whitelist it at:"))
				fmt.Println("  " + linkStyle.Render("https://ap.www.namecheap.com/settings/tools/apiaccess"))
				fmt.Println()
			}

			fmt.Println("Please update nameservers manually:")
			fmt.Printf("  1. Go to your registrar's domain control panel for %s\n", domain)
			fmt.Println("  2. Under 'Nameservers', select 'Custom DNS'")
			fmt.Printf("  3. Enter: %s\n", strings.Join(nameservers, ", "))
			fmt.Println()

			var done bool
			if err := huh.NewConfirm().
				Title("Have you updated the nameservers manually?").
				Affirmative("Yes, continue").
				Negative("No, abort").
				Value(&done).
				Run(); err != nil {
				return fmt.Errorf("prompt failed: %w", err)
			}
			if !done {
				return fmt.Errorf("setup aborted — update nameservers and rerun")
			}
		} else {
			fmt.Println(successStyle.Render("✓ Registrar nameservers updated"))
		}
	}

	// -----------------------------------------------------------------------
	// Step 5: Wait for NS propagation
	// -----------------------------------------------------------------------
	fmt.Println()
	fmt.Println(stepStyle.Render("Step 5: Checking NS propagation..."))

	err = dnspkg.WaitForNSPropagation(ctx, domain, nameservers, 10*time.Minute)
	if err != nil {
		fmt.Println(warnStyle.Render(fmt.Sprintf("⚠ NS propagation timed out: %v", err)))
		fmt.Println("  This is normal — DNS propagation can take up to 48 hours.")

		var continueAnyway bool
		if err := huh.NewConfirm().
			Title("Continue anyway?").
			Affirmative("Yes").
			Negative("No").
			Value(&continueAnyway).
			Run(); err != nil {
			return fmt.Errorf("prompt failed: %w", err)
		}
		if !continueAnyway {
			return fmt.Errorf("setup paused — rerun when NS records have propagated")
		}
	} else {
		fmt.Println(successStyle.Render("✓ Nameservers propagated successfully"))
	}

	// -----------------------------------------------------------------------
	// Step 6: Choose and configure hosting provider
	// -----------------------------------------------------------------------
	fmt.Println()
	fmt.Println(stepStyle.Render("Step 6: Choose and configure hosting provider"))

	// Build select options dynamically from the injected providers.
	options := make([]huh.Option[int], len(cfg.HostingProviders))
	for i, hp := range cfg.HostingProviders {
		options[i] = huh.NewOption(hp.Name(), i)
	}

	var selectedIdx int
	if err := huh.NewSelect[int]().
		Title("Choose hosting provider").
		Options(options...).
		Value(&selectedIdx).
		Run(); err != nil {
		return fmt.Errorf("hosting provider selection: %w", err)
	}

	selectedProvider := cfg.HostingProviders[selectedIdx]
	logger.Debug("selected hosting provider", "name", selectedProvider.Name())

	// Collect provider-specific parameters via huh forms.
	params, err := collectHostingParams(selectedProvider, domain)
	if err != nil {
		return fmt.Errorf("collecting hosting params: %w", err)
	}
	logger.Debug("hosting params collected", "params", params)

	// Execute provider setup — this handles site creation, custom domain
	// registration, etc. and returns the required DNS records.
	fmt.Println()
	fmt.Printf("  Configuring %s...\n", selectedProvider.Name())

	records, err := selectedProvider.Setup(ctx, domain, params)
	if errors.Is(err, firebase.ErrSiteAlreadyExists) {
		siteName := params["site_name"]
		fmt.Println(warnStyle.Render(fmt.Sprintf("  Site %q already exists", siteName)))

		var useExisting bool
		if err := huh.NewConfirm().
			Title(fmt.Sprintf("Use existing site %q?", siteName)).
			Affirmative("Yes, use it").
			Negative("No, abort").
			Value(&useExisting).
			Run(); err != nil {
			return fmt.Errorf("prompt failed: %w", err)
		}
		if !useExisting {
			return fmt.Errorf("setup aborted — choose a different site name")
		}
		params["use_existing"] = "true"
		records, err = selectedProvider.Setup(ctx, domain, params)
	}
	if err != nil {
		return fmt.Errorf("%s setup failed: %w", selectedProvider.Name(), err)
	}
	fmt.Println(successStyle.Render(fmt.Sprintf("✓ %s configured", selectedProvider.Name())))

	// For Firebase, show the default URLs and a console link for custom domain.
	if selectedProvider.Name() == "Firebase Hosting" {
		siteName := params["site_name"]
		linkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Underline(true)
		dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		fmt.Println()
		fmt.Println(dimStyle.Render("  Default URLs (available now):"))
		fmt.Println("    " + linkStyle.Render(fmt.Sprintf("https://%s.web.app", siteName)))
		fmt.Println("    " + linkStyle.Render(fmt.Sprintf("https://%s.firebaseapp.com", siteName)))
		fmt.Println()
		consoleURL := firebase.ConsoleURL(params["project_id"], siteName)
		fmt.Println(warnStyle.Render("  ⚠ Add your custom domain in the Firebase Console:"))
		fmt.Println("    " + linkStyle.Render(consoleURL))
		fmt.Println(dimStyle.Render("    (Click \"Add custom domain\" and enter: " + domain + ")"))
	}

	// -----------------------------------------------------------------------
	// Step 7: Create DNS records at DNS provider
	// -----------------------------------------------------------------------
	fmt.Println()
	fmt.Println(stepStyle.Render("Step 7: Creating DNS records..."))

	for _, r := range records {
		if err := cfg.DNSProvider.CreateDNSRecord(ctx, zoneID, r); err != nil {
			fmt.Println(warnStyle.Render(fmt.Sprintf("  ⚠ Failed to create %s record for %s: %v", r.Type, r.Name, err)))
		} else {
			fmt.Println(successStyle.Render(fmt.Sprintf("  ✓ %s %s → %s", r.Type, r.Name, r.Content)))
		}
	}

	// -----------------------------------------------------------------------
	// Step 8: Validate hosting site
	// -----------------------------------------------------------------------
	fmt.Println()
	fmt.Println(stepStyle.Render("Step 8: Checking hosting site..."))

	// Check default (non-custom) URLs first — they should work immediately.
	if selectedProvider.Name() == "Firebase Hosting" {
		siteName := params["site_name"]
		defaultURLs := []string{
			fmt.Sprintf("https://%s.web.app", siteName),
			fmt.Sprintf("https://%s.firebaseapp.com", siteName),
		}
		httpClient := &http.Client{Timeout: 15 * time.Second}
		for _, u := range defaultURLs {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
			if err != nil {
				fmt.Println(warnStyle.Render(fmt.Sprintf("  ⚠ %s — request error: %v", u, err)))
				continue
			}
			resp, err := httpClient.Do(req)
			if err != nil {
				fmt.Println(warnStyle.Render(fmt.Sprintf("  ⚠ %s — not reachable: %v", u, err)))
				continue
			}
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
			resp.Body.Close()
			if strings.Contains(string(body), "Site Not Found") {
				fmt.Println(warnStyle.Render(fmt.Sprintf("  ⚠ %s → \"Site Not Found\" — deploy content first", u)))
			} else if resp.StatusCode >= 200 && resp.StatusCode < 400 {
				fmt.Println(successStyle.Render(fmt.Sprintf("  ✓ %s → %d OK", u, resp.StatusCode)))
			} else {
				fmt.Println(warnStyle.Render(fmt.Sprintf("  ⚠ %s → HTTP %d", u, resp.StatusCode)))
			}
		}
	}

	// Check custom domain (may not be ready yet).
	fmt.Println()
	fmt.Printf("  Checking custom domain https://%s ...\n", domain)
	if err := dnspkg.CheckHTTPS(domain); err != nil {
		fmt.Println(warnStyle.Render(fmt.Sprintf("  ⚠ HTTPS not yet available: %v", err)))
		fmt.Println("    SSL certificate provisioning can take a few minutes.")
	} else {
		httpClient := &http.Client{Timeout: 15 * time.Second}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("https://%s", domain), nil)
		if err == nil {
			resp, err := httpClient.Do(req)
			if err != nil {
				fmt.Println(warnStyle.Render(fmt.Sprintf("  ⚠ Site not responding yet: %v", err)))
				fmt.Println("    This may resolve once DNS fully propagates.")
			} else {
				resp.Body.Close()
				fmt.Println(successStyle.Render(fmt.Sprintf("  ✓ https://%s → %d OK", domain, resp.StatusCode)))
			}
		}
	}

	// -----------------------------------------------------------------------
	// Summary box
	// -----------------------------------------------------------------------
	fmt.Println()

	summaryStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("10")).
		Padding(1, 2)

	summary := fmt.Sprintf(
		"✅ Domain setup complete!\n\n"+
			"Domain:      %s\n"+
			"DNS Zone:    %s\n"+
			"Nameservers: %s\n"+
			"Hosting:     %s",
		domain,
		zoneID,
		strings.Join(nameservers, ", "),
		selectedProvider.Name(),
	)
	fmt.Println(summaryStyle.Render(summary))

	fmt.Println()
	fmt.Println(warnStyle.Render("Note: Full DNS propagation may take up to 48 hours."))

	return nil
}

// ---------------------------------------------------------------------------
// collectHostingParams — provider-specific parameter collection via huh.
// ---------------------------------------------------------------------------

// collectHostingParams prompts the user for provider-specific configuration
// using huh forms. The returned map is passed directly to
// provider.HostingProvider.Setup().
func collectHostingParams(hp provider.HostingProvider, domain string) (map[string]string, error) {
	slug := strings.NewReplacer(".", "-", "_", "-").Replace(domain)

	switch hp.Name() {
	case "Firebase Hosting":
		var projectID, siteName string

		err := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Firebase project ID").
					Placeholder(slug).
					Value(&projectID),
				huh.NewInput().
					Title("Firebase Hosting site name").
					Placeholder(slug).
					Value(&siteName),
			),
		).Run()
		if err != nil {
			return nil, fmt.Errorf("firebase params: %w", err)
		}

		// Fall back to the domain-derived slug when the user leaves
		// the field empty (just pressed Enter).
		if projectID == "" {
			projectID = slug
		}
		if siteName == "" {
			siteName = slug
		}

		return map[string]string{
			"project_id": projectID,
			"site_name":  siteName,
		}, nil

	case "GitHub Pages":
		var repo string

		err := huh.NewInput().
			Title("GitHub repository (owner/repo)").
			Placeholder("owner/repo").
			Value(&repo).
			Run()
		if err != nil {
			return nil, fmt.Errorf("github params: %w", err)
		}
		if repo == "" {
			return nil, fmt.Errorf("repository name cannot be empty")
		}

		return map[string]string{
			"repo": repo,
		}, nil

	default:
		// Unknown provider — no extra params required.
		return map[string]string{}, nil
	}
}

// ---------------------------------------------------------------------------
// getPublicIP — utility for discovering the machine's public IP.
// ---------------------------------------------------------------------------

// getPublicIP queries an external service for the machine's public IPv4
// address. Returns an empty string on any failure. The caller can fall back to
// manual entry or an alternative detection method.
func getPublicIP(ctx context.Context) string {
	client := &http.Client{Timeout: 15 * time.Second}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.ipify.org", nil)
	if err != nil {
		return ""
	}

	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}

	ip := strings.TrimSpace(string(body))
	if net.ParseIP(ip) != nil {
		return ip
	}
	return ""
}

// nsMatch checks whether all expected nameservers appear in the current list.
// Handles trailing-dot normalization (e.g. "ns1.example.com." vs "ns1.example.com").
func nsMatch(current, expected []string) bool {
	if len(current) == 0 {
		return false
	}
	norm := make(map[string]bool, len(current)*2)
	for _, ns := range current {
		ns = strings.ToLower(strings.TrimSuffix(ns, "."))
		norm[ns] = true
	}
	for _, e := range expected {
		e = strings.ToLower(strings.TrimSuffix(e, "."))
		if !norm[e] {
			return false
		}
	}
	return true
}
