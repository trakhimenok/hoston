package dns

import (
	"context"
	"fmt"
	"net"
	"time"
)

// WaitForNSPropagation polls until the domain's NS records match expected values.
func WaitForNSPropagation(ctx context.Context, domain string, expectedNS []string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for NS propagation after %s", timeout)
		}

		current, err := net.LookupNS(domain)
		if err == nil && nsMatch(current, expectedNS) {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// WaitForRecord polls until a specific DNS record resolves.
func WaitForRecord(ctx context.Context, recordType, name, expected string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for %s record %s after %s", recordType, name, timeout)
		}

		var found bool
		switch recordType {
		case "A":
			ips, err := net.LookupHost(name)
			if err == nil {
				for _, ip := range ips {
					if ip == expected {
						found = true
						break
					}
				}
			}
		case "CNAME":
			cname, err := net.LookupCNAME(name)
			if err == nil && cname == expected+"." {
				found = true
			}
		case "TXT":
			txts, err := net.LookupTXT(name)
			if err == nil {
				for _, txt := range txts {
					if txt == expected {
						found = true
						break
					}
				}
			}
		}

		if found {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// CheckHTTPS verifies that a domain serves HTTPS.
func CheckHTTPS(domain string) error {
	conn, err := net.DialTimeout("tcp", domain+":443", 10*time.Second)
	if err != nil {
		return fmt.Errorf("HTTPS connection failed for %s: %w", domain, err)
	}
	conn.Close()
	return nil
}

func nsMatch(current []*net.NS, expected []string) bool {
	if len(current) == 0 {
		return false
	}
	currentMap := make(map[string]bool)
	for _, ns := range current {
		currentMap[ns.Host] = true
		// Also match without trailing dot.
		if len(ns.Host) > 0 && ns.Host[len(ns.Host)-1] == '.' {
			currentMap[ns.Host[:len(ns.Host)-1]] = true
		}
	}
	for _, e := range expected {
		if !currentMap[e] && !currentMap[e+"."] {
			return false
		}
	}
	return true
}
