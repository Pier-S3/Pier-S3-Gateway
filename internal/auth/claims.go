package auth

import (
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// ExtractUser extracts the preferred_username from JWT claims.
// Falls back to "sub" if preferred_username is not present.
func ExtractUser(claims jwt.MapClaims) string {
	if username, ok := claims["preferred_username"].(string); ok && username != "" {
		return username
	}
	if sub, ok := claims["sub"].(string); ok {
		return sub
	}
	return ""
}

// ExtractGroups extracts group memberships from JWT claims.
// Supports multiple claim paths:
//   - "groups" (direct array of strings)
//   - "realm_access.roles" (nested object)
//
// Leading "/" is stripped from each group name.
func ExtractGroups(claims jwt.MapClaims) []string {
	var groups []string

	// Try "groups" claim first (direct array)
	if g, ok := claims["groups"]; ok {
		if arr, ok := g.([]interface{}); ok {
			for _, item := range arr {
				if s, ok := item.(string); ok {
					groups = append(groups, stripLeadingSlash(s))
				}
			}
		}
	}

	// Also check realm_access.roles
	if ra, ok := claims["realm_access"]; ok {
		if raMap, ok := ra.(map[string]interface{}); ok {
			if roles, ok := raMap["roles"]; ok {
				if arr, ok := roles.([]interface{}); ok {
					for _, item := range arr {
						if s, ok := item.(string); ok {
							groups = append(groups, stripLeadingSlash(s))
						}
					}
				}
			}
		}
	}

	return groups
}

// stripLeadingSlash removes a leading "/" from a string.
func stripLeadingSlash(s string) string {
	return strings.TrimPrefix(s, "/")
}
