package config

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// wellKnownPath is the standard OIDC discovery document path appended to a base
// issuer URL when OIDC_DISCOVERY_URL does not already point at the document.
const wellKnownPath = "/.well-known/openid-configuration"

// maxDiscoveryBytes bounds the discovery document size. Real documents are a
// few KiB; the cap prevents memory exhaustion from a hostile/oversized body.
const maxDiscoveryBytes = 1 << 20 // 1 MiB

// discoveryDoc is the subset of the OIDC discovery document we consume.
type discoveryDoc struct {
	Issuer  string `json:"issuer"`
	JWKSURL string `json:"jwks_uri"`
}

// discoverOIDC fetches an OIDC discovery document and returns its issuer and
// jwks_uri. The argument may be either the full document URL (containing
// "/.well-known/") or a base issuer URL, in which case the well-known path is
// appended.
func discoverOIDC(discoveryURL string) (discoveryDoc, error) {
	url := discoveryURL
	if !strings.Contains(url, "/.well-known/") {
		url = strings.TrimRight(url, "/") + wellKnownPath
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return discoveryDoc{}, fmt.Errorf("fetching %s: %w", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxDiscoveryBytes))
	if err != nil {
		return discoveryDoc{}, fmt.Errorf("reading discovery response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return discoveryDoc{}, fmt.Errorf("discovery endpoint %s returned %d", url, resp.StatusCode)
	}

	var doc discoveryDoc
	if err := json.Unmarshal(body, &doc); err != nil {
		return discoveryDoc{}, fmt.Errorf("parsing discovery document: %w", err)
	}
	if doc.Issuer == "" && doc.JWKSURL == "" {
		return discoveryDoc{}, fmt.Errorf("discovery document from %s contained neither issuer nor jwks_uri", url)
	}
	return doc, nil
}
