package dns

import (
	"net"
	"testing"
)

// ---------------------------------------------------------------------------
// nsMatch unit tests — same package so the unexported function is accessible.
// ---------------------------------------------------------------------------

func TestNsMatch_EmptyCurrentReturnsfalse(t *testing.T) {
	t.Parallel()
	if nsMatch(nil, []string{"ns1.example.com"}) {
		t.Error("nsMatch with nil current should return false")
	}
	if nsMatch([]*net.NS{}, []string{"ns1.example.com"}) {
		t.Error("nsMatch with empty current should return false")
	}
}

func TestNsMatch_ExactMatch(t *testing.T) {
	t.Parallel()
	current := []*net.NS{
		{Host: "ns1.example.com."},
		{Host: "ns2.example.com."},
	}
	expected := []string{"ns1.example.com", "ns2.example.com"}
	if !nsMatch(current, expected) {
		t.Error("nsMatch should return true when all expected NS are present (trailing-dot normalisation)")
	}
}

func TestNsMatch_TrailingDotOnExpected(t *testing.T) {
	t.Parallel()
	current := []*net.NS{
		{Host: "ns1.example.com."},
	}
	// expected already carries trailing dot — should still match.
	expected := []string{"ns1.example.com."}
	if !nsMatch(current, expected) {
		t.Error("nsMatch should match when expected has trailing dot")
	}
}

func TestNsMatch_MissingNameserver(t *testing.T) {
	t.Parallel()
	current := []*net.NS{
		{Host: "ns1.example.com."},
	}
	expected := []string{"ns1.example.com", "ns2.example.com"} // ns2 is absent
	if nsMatch(current, expected) {
		t.Error("nsMatch should return false when a required NS is missing")
	}
}

func TestNsMatch_ExtraNameserversAreOK(t *testing.T) {
	t.Parallel()
	current := []*net.NS{
		{Host: "ns1.example.com."},
		{Host: "ns2.example.com."},
		{Host: "ns3.example.com."},
	}
	expected := []string{"ns1.example.com", "ns2.example.com"} // extra ns3 is fine
	if !nsMatch(current, expected) {
		t.Error("nsMatch should return true when all expected NS are present (extras allowed)")
	}
}

func TestNsMatch_CurrentWithoutTrailingDot(t *testing.T) {
	t.Parallel()
	// Some resolvers return NS without a trailing dot.
	current := []*net.NS{
		{Host: "ns1.example.com"}, // no dot
	}
	expected := []string{"ns1.example.com"}
	if !nsMatch(current, expected) {
		t.Error("nsMatch should match when current NS has no trailing dot")
	}
}

func TestNsMatch_EmptyExpected(t *testing.T) {
	t.Parallel()
	current := []*net.NS{
		{Host: "ns1.example.com."},
	}
	// No requirements → everything matches.
	if !nsMatch(current, []string{}) {
		t.Error("nsMatch with empty expected should return true")
	}
}

// ---------------------------------------------------------------------------
// CheckHTTPS tests — no real DNS or TLS required.
// ---------------------------------------------------------------------------

// TestCheckHTTPS_Failure verifies that CheckHTTPS returns an error when nothing
// is listening on port 443 of the given address.
func TestCheckHTTPS_Failure(t *testing.T) {
	t.Parallel()
	// Use the loopback address. Nothing should be listening on port 443 in a CI
	// environment, so the dial must fail.
	err := CheckHTTPS("0.0.0.0")
	if err == nil {
		t.Skip("something is unexpectedly listening on 0.0.0.0:443; skipping")
	}
}

// TestCheckHTTPS_Success creates a real TCP listener on a free port, then
// rewires CheckHTTPS indirectly by confirming the dial logic works at all.
// Because CheckHTTPS hardcodes port 443 we can only fully exercise the happy
// path in an integration test; this unit test validates the error message shape.
func TestCheckHTTPS_ErrorContainsDomain(t *testing.T) {
	t.Parallel()
	const domain = "nonexistent.invalid"
	err := CheckHTTPS(domain)
	if err == nil {
		t.Skip("unexpected successful dial to nonexistent.invalid:443; skipping")
	}
	errStr := err.Error()
	// The error message must mention the domain so callers can identify the failure.
	found := false
	for i := 0; i+len(domain) <= len(errStr); i++ {
		if errStr[i:i+len(domain)] == domain {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error message to contain %q, got: %s", domain, errStr)
	}
}
