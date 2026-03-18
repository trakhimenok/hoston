package wizard

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	cfclient "github.com/trakhimenok/hostme/internal/cloudflare"
	dnspkg "github.com/trakhimenok/hostme/internal/dns"
	"github.com/trakhimenok/hostme/internal/firebase"
	ghpages "github.com/trakhimenok/hostme/internal/github"
	"github.com/trakhimenok/hostme/internal/keychain"
	"github.com/trakhimenok/hostme/internal/namecheap"
)

// RunSetup orchestrates the full domain setup wizard.
func RunSetup(ctx context.Context, domain string, verbose bool) error {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("🚀 Setting up domain: %s\n\n", domain)

	// Step 1: Verify authentication
	fmt.Println("Step 1: Verifying credentials...")
	cfToken, err := keychain.GetCloudflareToken()
	if err != nil {
		return fmt.Errorf("CloudFlare credentials not found. Run: hostme auth cloudflare")
	}
	ncAPIUser, ncAPIKey, ncUsername, err := keychain.GetNamecheapCredentials()
	if err != nil {
		return fmt.Errorf("NameCheap credentials not found. Run: hostme auth namecheap")
	}
	fmt.Println("✓ Credentials verified")

	// Step 2: Add domain to CloudFlare
	fmt.Println("\nStep 2: Adding domain to CloudFlare...")
	cf, err := cfclient.NewClient(cfToken)
	if err != nil {
		return fmt.Errorf("failed to create CloudFlare client: %w", err)
	}

	var zoneID string
	var nameservers []string

	// Check if zone already exists.
	zoneID, nameservers, err = cf.GetZoneByDomain(ctx, domain)
	if err == nil {
		fmt.Printf("✓ Domain already exists in CloudFlare (zone: %s)\n", zoneID)
	} else {
		zoneID, err = cf.AddZone(ctx, domain)
		if err != nil {
			return fmt.Errorf("failed to add zone: %w", err)
		}
		fmt.Printf("✓ Domain added to CloudFlare (zone: %s)\n", zoneID)
	}

	// Step 3: Get CloudFlare nameservers
	if len(nameservers) == 0 {
		nameservers, err = cf.GetNameservers(ctx, zoneID)
		if err != nil {
			return fmt.Errorf("failed to get nameservers: %w", err)
		}
	}
	fmt.Printf("\nStep 3: CloudFlare nameservers:\n")
	for _, ns := range nameservers {
		fmt.Printf("  • %s\n", ns)
	}

	// Step 4: Update NameCheap nameservers
	fmt.Println("\nStep 4: Updating NameCheap nameservers...")
	clientIP := getPublicIP()
	if clientIP == "" {
		fmt.Print("Could not detect public IP. Enter your public IP: ")
		line, _ := reader.ReadString('\n')
		clientIP = strings.TrimSpace(line)
	}

	nc := namecheap.NewClient(ncAPIUser, ncAPIKey, ncUsername, clientIP, false)
	err = nc.SetCustomNameservers(domain, nameservers)
	if err != nil {
		fmt.Printf("\n⚠ Automatic NS update failed: %v\n", err)
		fmt.Println("\nPlease update nameservers manually:")
		fmt.Printf("  1. Go to: https://ap.www.namecheap.com/domains/domaincontrolpanel/%s/domain\n", domain)
		fmt.Println("  2. Under 'Nameservers', select 'Custom DNS'")
		fmt.Printf("  3. Enter: %s\n", strings.Join(nameservers, ", "))
		fmt.Print("\nPress Enter when done...")
		_, _ = reader.ReadString('\n')
	} else {
		fmt.Println("✓ NameCheap nameservers updated")
	}

	// Step 5: Wait for NS propagation
	fmt.Println("\nStep 5: Checking NS propagation...")
	err = dnspkg.WaitForNSPropagation(ctx, domain, nameservers, 10*time.Minute)
	if err != nil {
		fmt.Printf("⚠ NS propagation timed out: %v\n", err)
		fmt.Println("This is normal — DNS propagation can take up to 48 hours.")
		fmt.Print("Continue anyway? (y/n): ")
		line, _ := reader.ReadString('\n')
		if strings.TrimSpace(strings.ToLower(line)) != "y" {
			return fmt.Errorf("setup paused — rerun when NS records have propagated")
		}
	}

	// Step 6: Choose hosting provider
	fmt.Println("\nStep 6: Choose hosting provider")
	fmt.Println("  1. Firebase Hosting")
	fmt.Println("  2. GitHub Pages")
	fmt.Print("Select (1/2): ")
	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(choice)

	switch choice {
	case "1":
		err = setupFirebase(ctx, cf, zoneID, domain, reader)
	case "2":
		err = setupGitHubPages(ctx, cf, zoneID, domain, reader)
	default:
		return fmt.Errorf("invalid choice: %s", choice)
	}
	if err != nil {
		return err
	}

	// Final validation
	fmt.Println("\nStep 10: Validating HTTPS...")
	err = dnspkg.CheckHTTPS(domain)
	if err != nil {
		fmt.Printf("⚠ HTTPS not yet available: %v\n", err)
		fmt.Println("SSL certificate provisioning can take a few minutes.")
	}

	// Validate site loads
	fmt.Println("\nStep 11: Checking site responds...")
	resp, err := http.Get(fmt.Sprintf("https://%s", domain))
	if err != nil {
		fmt.Printf("⚠ Site not responding yet: %v\n", err)
		fmt.Println("This may resolve once DNS and SSL fully propagate.")
	} else {
		resp.Body.Close()
		fmt.Printf("✓ Site responds with status %d\n", resp.StatusCode)
	}

	// Report completion
	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Printf("✅ Domain setup complete for %s\n", domain)
	fmt.Println(strings.Repeat("=", 50))
	fmt.Println("\nSummary:")
	fmt.Printf("  Domain:      %s\n", domain)
	fmt.Printf("  DNS:         CloudFlare (zone: %s)\n", zoneID)
	fmt.Printf("  Nameservers: %s\n", strings.Join(nameservers, ", "))
	fmt.Printf("  Hosting:     %s\n", map[string]string{"1": "Firebase", "2": "GitHub Pages"}[choice])
	fmt.Println("\nNote: Full DNS propagation may take up to 48 hours.")

	return nil
}

func setupFirebase(ctx context.Context, cf *cfclient.Client, zoneID, domain string, reader *bufio.Reader) error {
	slug := strings.ReplaceAll(strings.ReplaceAll(domain, ".", "-"), "_", "-")

	fmt.Printf("\nStep 7a: Firebase project ID [suggest: %s]: ", slug)
	line, _ := reader.ReadString('\n')
	projectID := strings.TrimSpace(line)
	if projectID == "" {
		projectID = slug
	}

	fmt.Printf("Step 7b: Firebase hosting site name [suggest: %s]: ", slug)
	line, _ = reader.ReadString('\n')
	siteName := strings.TrimSpace(line)
	if siteName == "" {
		siteName = slug
	}

	// Check/create hosting site
	fmt.Println("\nStep 7c: Checking Firebase Hosting site...")
	exists, err := firebase.SiteExists(projectID, siteName)
	if err != nil {
		fmt.Printf("⚠ Could not check site existence: %v\n", err)
	}

	if !exists {
		fmt.Printf("Creating hosting site: %s...\n", siteName)
		err = firebase.CreateSite(projectID, siteName)
		if err != nil {
			fmt.Printf("⚠ Auto-create failed: %v\n", err)
			fmt.Printf("Please create manually: firebase hosting:sites:create %s --project %s\n", siteName, projectID)
			fmt.Print("Press Enter when done...")
			_, _ = reader.ReadString('\n')
		}
	} else {
		fmt.Printf("✓ Hosting site %s already exists\n", siteName)
	}

	// Validate auto URL
	autoURL := firebase.GetAutoURL(siteName)
	fmt.Printf("\nStep 7d: Hosting URL: %s\n", autoURL)

	// Add custom domain
	fmt.Println("\nStep 8: Adding custom domain to Firebase...")
	records, err := firebase.AddCustomDomain(projectID, siteName, domain)
	if err != nil {
		fmt.Printf("⚠ Auto-add failed: %v\n", err)
		fmt.Printf("Please add manually: firebase hosting:custom-domains:create %s --project %s --site %s\n", domain, projectID, siteName)
		fmt.Print("Press Enter when done, then enter the required DNS records.\n")
		_, _ = reader.ReadString('\n')
	}

	// Use default Firebase records if we couldn't parse them.
	if len(records) == 0 {
		records = func() []firebase.DNSRecord {
			fr := firebase.GetRequiredDNSRecords(domain)
			return fr
		}()
	}

	// Step 9: Update CloudFlare DNS
	fmt.Println("\nStep 9: Creating DNS records in CloudFlare...")
	for _, r := range records {
		err := cf.CreateDNSRecord(ctx, zoneID, cfclient.DNSRecord{
			Type:    r.Type,
			Name:    r.Name,
			Content: r.Content,
			Proxied: false,
		})
		if err != nil {
			fmt.Printf("  ⚠ Failed to create %s record for %s: %v\n", r.Type, r.Name, err)
		} else {
			fmt.Printf("  ✓ %s %s → %s\n", r.Type, r.Name, r.Content)
		}
	}

	return nil
}

func setupGitHubPages(ctx context.Context, cf *cfclient.Client, zoneID, domain string, reader *bufio.Reader) error {
	fmt.Print("\nStep 7a: GitHub repository (owner/repo): ")
	line, _ := reader.ReadString('\n')
	repo := strings.TrimSpace(line)
	if repo == "" {
		return fmt.Errorf("repository name cannot be empty")
	}

	// Enable Pages
	fmt.Println("\nStep 7b: Enabling GitHub Pages...")
	err := ghpages.EnablePages(repo)
	if err != nil {
		fmt.Printf("⚠ Auto-enable failed: %v\n", err)
		fmt.Printf("Please enable manually at: https://github.com/%s/settings/pages\n", repo)
		fmt.Print("Press Enter when done...")
		_, _ = reader.ReadString('\n')
	}

	// Set custom domain
	fmt.Println("\nStep 8: Setting custom domain...")
	err = ghpages.SetCustomDomain(repo, domain)
	if err != nil {
		fmt.Printf("⚠ Auto-set failed: %v\n", err)
		fmt.Printf("Please set manually at: https://github.com/%s/settings/pages\n", repo)
		fmt.Print("Press Enter when done...")
		_, _ = reader.ReadString('\n')
	}

	// Step 9: Update CloudFlare DNS
	fmt.Println("\nStep 9: Creating DNS records in CloudFlare...")
	records := ghpages.GetRequiredDNSRecords(domain)
	for _, r := range records {
		err := cf.CreateDNSRecord(ctx, zoneID, cfclient.DNSRecord{
			Type:    r.Type,
			Name:    r.Name,
			Content: r.Content,
			Proxied: false,
		})
		if err != nil {
			fmt.Printf("  ⚠ Failed to create %s record for %s: %v\n", r.Type, r.Name, err)
		} else {
			fmt.Printf("  ✓ %s %s → %s\n", r.Type, r.Name, r.Content)
		}
	}

	return nil
}

func getPublicIP() string {
	resp, err := http.Get("https://api.ipify.org")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	buf := make([]byte, 64)
	n, _ := resp.Body.Read(buf)
	ip := strings.TrimSpace(string(buf[:n]))
	if net.ParseIP(ip) != nil {
		return ip
	}
	return ""
}
