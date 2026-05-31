package auth

import (
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

// TestExtractUser tests the ExtractUser function with various claim structures.
func TestExtractUser(t *testing.T) {
	tests := []struct {
		name           string
		claims         jwt.MapClaims
		expectUsername string
	}{
		{
			name: "preferred_username present",
			claims: jwt.MapClaims{
				"preferred_username": "alice@example.com",
			},
			expectUsername: "alice@example.com",
		},
		{
			name: "preferred_username and sub both present preferred wins",
			claims: jwt.MapClaims{
				"preferred_username": "alice@example.com",
				"sub":                "user-123",
			},
			expectUsername: "alice@example.com",
		},
		{
			name: "only sub present",
			claims: jwt.MapClaims{
				"sub": "user-123",
			},
			expectUsername: "user-123",
		},
		{
			name:           "empty claims",
			claims:         jwt.MapClaims{},
			expectUsername: "",
		},
		{
			name: "preferred_username empty string falls back to sub",
			claims: jwt.MapClaims{
				"preferred_username": "",
				"sub":                "user-456",
			},
			expectUsername: "user-456",
		},
		{
			name: "preferred_username wrong type",
			claims: jwt.MapClaims{
				"preferred_username": 123,
				"sub":                "user-789",
			},
			expectUsername: "user-789",
		},
		{
			name: "sub wrong type",
			claims: jwt.MapClaims{
				"sub": 456,
			},
			expectUsername: "",
		},
		{
			name: "nil claims value",
			claims: jwt.MapClaims{
				"preferred_username": nil,
				"sub":                nil,
			},
			expectUsername: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractUser(tt.claims)
			assert.Equal(t, tt.expectUsername, result, "username mismatch")
		})
	}
}

// TestExtractGroups tests the ExtractGroups function with various claim structures.
func TestExtractGroups(t *testing.T) {
	tests := []struct {
		name         string
		claims       jwt.MapClaims
		expectGroups []string
	}{
		{
			name: "groups array present",
			claims: jwt.MapClaims{
				"groups": []interface{}{"reports-ro", "dev-rw"},
			},
			expectGroups: []string{"reports-ro", "dev-rw"},
		},
		{
			name: "groups with leading slashes stripped",
			claims: jwt.MapClaims{
				"groups": []interface{}{"/reports-ro", "/dev-rw"},
			},
			expectGroups: []string{"reports-ro", "dev-rw"},
		},
		{
			name: "realm_access.roles present",
			claims: jwt.MapClaims{
				"realm_access": map[string]interface{}{
					"roles": []interface{}{"admin", "user"},
				},
			},
			expectGroups: []string{"admin", "user"},
		},
		{
			name: "realm_access.roles with leading slashes",
			claims: jwt.MapClaims{
				"realm_access": map[string]interface{}{
					"roles": []interface{}{"/admin", "/user"},
				},
			},
			expectGroups: []string{"admin", "user"},
		},
		{
			name: "both groups and realm_access merged",
			claims: jwt.MapClaims{
				"groups": []interface{}{"reports-ro"},
				"realm_access": map[string]interface{}{
					"roles": []interface{}{"admin"},
				},
			},
			expectGroups: []string{"reports-ro", "admin"},
		},
		{
			name: "groups array with mixed leading slashes",
			claims: jwt.MapClaims{
				"groups": []interface{}{"reports-ro", "/dev-rw", "data-ro"},
			},
			expectGroups: []string{"reports-ro", "dev-rw", "data-ro"},
		},
		{
			name:         "empty claims",
			claims:       jwt.MapClaims{},
			expectGroups: []string(nil),
		},
		{
			name: "groups wrong type",
			claims: jwt.MapClaims{
				"groups": "not-an-array",
			},
			expectGroups: []string(nil),
		},
		{
			name: "groups contains non-string values",
			claims: jwt.MapClaims{
				"groups": []interface{}{"reports-ro", 123, "dev-rw"},
			},
			expectGroups: []string{"reports-ro", "dev-rw"},
		},
		{
			name: "realm_access wrong type",
			claims: jwt.MapClaims{
				"realm_access": "not-an-object",
			},
			expectGroups: []string(nil),
		},
		{
			name: "realm_access.roles wrong type",
			claims: jwt.MapClaims{
				"realm_access": map[string]interface{}{
					"roles": "not-an-array",
				},
			},
			expectGroups: []string(nil),
		},
		{
			name: "empty groups array",
			claims: jwt.MapClaims{
				"groups": []interface{}{},
			},
			expectGroups: []string(nil),
		},
		{
			name: "empty realm_access roles",
			claims: jwt.MapClaims{
				"realm_access": map[string]interface{}{
					"roles": []interface{}{},
				},
			},
			expectGroups: []string(nil),
		},
		{
			name: "single group",
			claims: jwt.MapClaims{
				"groups": []interface{}{"reports-ro"},
			},
			expectGroups: []string{"reports-ro"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractGroups(tt.claims)
			assert.Equal(t, tt.expectGroups, result, "groups mismatch")
		})
	}
}
