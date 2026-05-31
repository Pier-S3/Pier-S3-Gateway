package auth

import (
	"sort"
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

// ClaimMapper resolves the identity (username) and authorization groups from a
// verified token's claims. The zero value reproduces the default,
// Keycloak-compatible behavior: username from preferred_username (then sub),
// and groups from the "groups" array merged with "realm_access.roles". Setting
// UsernameClaim or GroupsClaim overrides the respective default with a
// configurable dotted-path lookup, so any compliant OIDC provider can be
// supported by configuration alone.
type ClaimMapper struct {
	// UsernameClaim is a dotted path to the identity claim (e.g. "email" or
	// "preferred_username"). Empty selects the default preferred_username->sub
	// behavior. Note: a claim whose key literally contains "." cannot be
	// addressed via the dotted path.
	UsernameClaim string
	// GroupsClaim is a dotted path to the groups/roles claim (e.g. "groups",
	// "roles", "realm_access.roles", "resource_access.myclient.roles"). Empty
	// selects the default groups + realm_access.roles merge.
	GroupsClaim string
}

// User returns the identity for the given claims, honoring UsernameClaim when
// set and otherwise falling back to the default extraction. When a configured
// username claim is missing or not a non-empty string, it falls back to "sub"
// so a misconfigured mapping still yields a stable identifier rather than "".
func (m ClaimMapper) User(claims jwt.MapClaims) string {
	if m.UsernameClaim == "" {
		return ExtractUser(claims)
	}
	if v, ok := lookupClaim(claims, m.UsernameClaim); ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	if sub, ok := claims["sub"].(string); ok {
		return sub
	}
	return ""
}

// Groups returns the authorization groups for the given claims, honoring
// GroupsClaim when set and otherwise falling back to the default extraction.
// The configured claim may be a string array, a string-keyed object (whose keys
// are treated as group names, e.g. Zitadel project roles), or a single string.
func (m ClaimMapper) Groups(claims jwt.MapClaims) []string {
	if m.GroupsClaim == "" {
		return ExtractGroups(claims)
	}
	v, ok := lookupClaim(claims, m.GroupsClaim)
	if !ok {
		return nil
	}
	return toGroupStrings(v)
}

// lookupClaim resolves a dotted path (e.g. "realm_access.roles") against nested
// JSON objects in the claims, returning the value and whether it was found.
func lookupClaim(claims jwt.MapClaims, path string) (interface{}, bool) {
	var cur interface{} = map[string]interface{}(claims)
	for _, part := range strings.Split(path, ".") {
		m, ok := cur.(map[string]interface{})
		if !ok {
			return nil, false
		}
		cur, ok = m[part]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

// toGroupStrings normalizes a claim value into a list of group names. It
// accepts a string array, a string-keyed object (keys become group names), or a
// single string. Leading "/" is stripped from each name. Object keys are sorted
// for deterministic output.
func toGroupStrings(v interface{}) []string {
	var out []string
	switch t := v.(type) {
	case []interface{}:
		for _, item := range t {
			if s, ok := item.(string); ok {
				out = append(out, stripLeadingSlash(s))
			}
		}
	case []string:
		for _, s := range t {
			out = append(out, stripLeadingSlash(s))
		}
	case map[string]interface{}:
		for k := range t {
			out = append(out, stripLeadingSlash(k))
		}
		sort.Strings(out)
	case string:
		if t != "" {
			out = append(out, stripLeadingSlash(t))
		}
	}
	return out
}

// stripLeadingSlash removes a leading "/" from a string.
func stripLeadingSlash(s string) string {
	return strings.TrimPrefix(s, "/")
}
