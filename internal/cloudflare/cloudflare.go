package cloudflare

import (
	"context"
	"fmt"

	cf "github.com/cloudflare/cloudflare-go"
	"github.com/trakhimenok/hoston/internal/provider"
)

// Compile-time check that *Client satisfies provider.DNSProvider.
var _ provider.DNSProvider = (*Client)(nil)

// Client wraps the CloudFlare API.
type Client struct {
	api *cf.API
}

// NewClient creates a CloudFlare client from an API token.
func NewClient(apiToken string) (*Client, error) {
	api, err := cf.NewWithAPIToken(apiToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create CloudFlare client: %w", err)
	}
	return &Client{api: api}, nil
}

// AddZone adds a domain to CloudFlare and returns the zone ID.
func (c *Client) AddZone(ctx context.Context, domain string) (zoneID string, err error) {
	zone, err := c.api.CreateZone(ctx, domain, false, cf.Account{}, "full")
	if err != nil {
		return "", fmt.Errorf("failed to add zone %s: %w", domain, err)
	}
	return zone.ID, nil
}

// GetZoneByDomain returns an existing zone for the domain.
func (c *Client) GetZoneByDomain(ctx context.Context, domain string) (zoneID string, nameservers []string, err error) {
	zones, err := c.api.ListZones(ctx, domain)
	if err != nil {
		return "", nil, fmt.Errorf("failed to list zones: %w", err)
	}
	for _, z := range zones {
		if z.Name == domain {
			return z.ID, z.NameServers, nil
		}
	}
	return "", nil, fmt.Errorf("zone not found for %s", domain)
}

// GetNameservers returns the CloudFlare nameservers for a zone.
func (c *Client) GetNameservers(ctx context.Context, zoneID string) ([]string, error) {
	zone, err := c.api.ZoneDetails(ctx, zoneID)
	if err != nil {
		return nil, fmt.Errorf("failed to get zone details: %w", err)
	}
	return zone.NameServers, nil
}

// CreateDNSRecord creates a DNS record in the specified zone.
func (c *Client) CreateDNSRecord(ctx context.Context, zoneID string, record provider.DNSRecord) error {
	ttl := record.TTL
	if ttl == 0 {
		ttl = 1 // 1 = automatic
	}
	rc := cf.ZoneIdentifier(zoneID)
	_, err := c.api.CreateDNSRecord(ctx, rc, cf.CreateDNSRecordParams{
		Type:    record.Type,
		Name:    record.Name,
		Content: record.Content,
		TTL:     ttl,
		Proxied: &record.Proxied,
	})
	if err != nil {
		return fmt.Errorf("failed to create DNS record %s %s: %w", record.Type, record.Name, err)
	}
	return nil
}

// ListDNSRecords returns all DNS records for a zone.
func (c *Client) ListDNSRecords(ctx context.Context, zoneID string) ([]provider.DNSRecord, error) {
	rc := cf.ZoneIdentifier(zoneID)
	cfRecords, _, err := c.api.ListDNSRecords(ctx, rc, cf.ListDNSRecordsParams{})
	if err != nil {
		return nil, fmt.Errorf("failed to list DNS records: %w", err)
	}
	records := make([]provider.DNSRecord, len(cfRecords))
	for i, r := range cfRecords {
		proxied := r.Proxied != nil && *r.Proxied
		records[i] = provider.DNSRecord{
			Type:    r.Type,
			Name:    r.Name,
			Content: r.Content,
			TTL:     r.TTL,
			Proxied: proxied,
		}
	}
	return records, nil
}

// DeleteDNSRecord removes a DNS record.
func (c *Client) DeleteDNSRecord(ctx context.Context, zoneID, recordID string) error {
	rc := cf.ZoneIdentifier(zoneID)
	return c.api.DeleteDNSRecord(ctx, rc, recordID)
}
