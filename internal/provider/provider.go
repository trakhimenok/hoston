// Package provider defines shared types and interfaces for domain registrars,
// DNS providers, and hosting providers. Implementations live in their own packages.
package provider

import "context"

// DNSRecord represents a DNS record across all providers.
type DNSRecord struct {
	Type    string
	Name    string
	Content string
	TTL     int
	Proxied bool
}

// Registrar manages domain registration and nameserver delegation.
type Registrar interface {
	// SetCustomNameservers updates the domain's nameservers at the registrar.
	SetCustomNameservers(domain string, nameservers []string) error

	// GetNameservers returns the currently configured nameservers for a domain.
	GetNameservers(domain string) ([]string, error)
}

// DNSProvider manages DNS zones and records.
type DNSProvider interface {
	// AddZone creates a new DNS zone for the domain and returns a zone identifier.
	AddZone(ctx context.Context, domain string) (zoneID string, err error)

	// GetZoneByDomain returns an existing zone's ID and nameservers.
	GetZoneByDomain(ctx context.Context, domain string) (zoneID string, nameservers []string, err error)

	// GetNameservers returns the provider's nameservers for a zone.
	GetNameservers(ctx context.Context, zoneID string) ([]string, error)

	// CreateDNSRecord creates a DNS record in the specified zone.
	CreateDNSRecord(ctx context.Context, zoneID string, record DNSRecord) error

	// ListDNSRecords returns all DNS records for a zone.
	ListDNSRecords(ctx context.Context, zoneID string) ([]DNSRecord, error)

	// DeleteDNSRecord removes a DNS record by ID.
	DeleteDNSRecord(ctx context.Context, zoneID, recordID string) error
}

// HostingProvider manages hosting setup and custom domain configuration.
type HostingProvider interface {
	// Name returns a human-readable name for the provider (e.g. "Firebase Hosting").
	Name() string

	// Setup performs the initial hosting setup and returns required DNS records.
	// The params map carries provider-specific configuration (e.g. project ID, repo name).
	Setup(ctx context.Context, domain string, params map[string]string) ([]DNSRecord, error)
}
