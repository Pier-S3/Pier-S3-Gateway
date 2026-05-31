package auth

import (
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

// TestClaimMapperUser covers the configurable username claim plus the default
// (zero-value) behavior and the sub fallback for a misconfigured claim.
func TestClaimMapperUser(t *testing.T) {
	tests := []struct {
		name   string
		mapper ClaimMapper
		claims jwt.MapClaims
		want   string
	}{
		{
			name:   "zero value uses default preferred_username",
			mapper: ClaimMapper{},
			claims: jwt.MapClaims{"preferred_username": "alice", "sub": "u-1"},
			want:   "alice",
		},
		{
			name:   "configured email claim",
			mapper: ClaimMapper{UsernameClaim: "email"},
			claims: jwt.MapClaims{"email": "bob@example.com", "sub": "u-2"},
			want:   "bob@example.com",
		},
		{
			name:   "dotted username path",
			mapper: ClaimMapper{UsernameClaim: "profile.login"},
			claims: jwt.MapClaims{"profile": map[string]interface{}{"login": "carol"}},
			want:   "carol",
		},
		{
			name:   "missing configured claim falls back to sub",
			mapper: ClaimMapper{UsernameClaim: "email"},
			claims: jwt.MapClaims{"sub": "u-3"},
			want:   "u-3",
		},
		{
			name:   "empty configured claim value falls back to sub",
			mapper: ClaimMapper{UsernameClaim: "email"},
			claims: jwt.MapClaims{"email": "", "sub": "u-4"},
			want:   "u-4",
		},
		{
			name:   "non-string configured claim falls back to sub",
			mapper: ClaimMapper{UsernameClaim: "email"},
			claims: jwt.MapClaims{"email": 123, "sub": "u-5"},
			want:   "u-5",
		},
		{
			name:   "missing claim and no sub yields empty",
			mapper: ClaimMapper{UsernameClaim: "email"},
			claims: jwt.MapClaims{},
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.mapper.User(tt.claims))
		})
	}
}

// TestClaimMapperGroups covers the configurable groups claim across array,
// nested, object-keyed (Zitadel-style), and string shapes, plus the default.
func TestClaimMapperGroups(t *testing.T) {
	tests := []struct {
		name   string
		mapper ClaimMapper
		claims jwt.MapClaims
		want   []string
	}{
		{
			name:   "zero value uses default groups + realm_access.roles",
			mapper: ClaimMapper{},
			claims: jwt.MapClaims{
				"groups":       []interface{}{"reports-ro"},
				"realm_access": map[string]interface{}{"roles": []interface{}{"admin"}},
			},
			want: []string{"reports-ro", "admin"},
		},
		{
			name:   "configured flat groups array",
			mapper: ClaimMapper{GroupsClaim: "roles"},
			claims: jwt.MapClaims{"roles": []interface{}{"data-rw", "/logs-ro"}},
			want:   []string{"data-rw", "logs-ro"},
		},
		{
			name:   "dotted path to nested roles",
			mapper: ClaimMapper{GroupsClaim: "resource_access.s3-proxy.roles"},
			claims: jwt.MapClaims{
				"resource_access": map[string]interface{}{
					"s3-proxy": map[string]interface{}{
						"roles": []interface{}{"bucket-a-ro", "bucket-b-rw"},
					},
				},
			},
			want: []string{"bucket-a-ro", "bucket-b-rw"},
		},
		{
			name:   "object-keyed roles (Zitadel-style) sorted",
			mapper: ClaimMapper{GroupsClaim: "urn:zitadel:roles"},
			claims: jwt.MapClaims{
				"urn:zitadel:roles": map[string]interface{}{
					"data-rw":  map[string]interface{}{"org1": "x"},
					"logs-ro":  map[string]interface{}{"org1": "y"},
					"audit-ro": map[string]interface{}{"org1": "z"},
				},
			},
			want: []string{"audit-ro", "data-rw", "logs-ro"},
		},
		{
			name:   "single string value",
			mapper: ClaimMapper{GroupsClaim: "role"},
			claims: jwt.MapClaims{"role": "/single-ro"},
			want:   []string{"single-ro"},
		},
		{
			name:   "missing configured claim yields nil",
			mapper: ClaimMapper{GroupsClaim: "roles"},
			claims: jwt.MapClaims{},
			want:   nil,
		},
		{
			name:   "broken dotted path yields nil",
			mapper: ClaimMapper{GroupsClaim: "a.b.c"},
			claims: jwt.MapClaims{"a": map[string]interface{}{"b": "not-an-object"}},
			want:   nil,
		},
		{
			name:   "empty string value yields empty",
			mapper: ClaimMapper{GroupsClaim: "role"},
			claims: jwt.MapClaims{"role": ""},
			want:   nil,
		},
		{
			name:   "array with non-string entries skipped",
			mapper: ClaimMapper{GroupsClaim: "roles"},
			claims: jwt.MapClaims{"roles": []interface{}{"keep-ro", 7, "keep-rw"}},
			want:   []string{"keep-ro", "keep-rw"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.mapper.Groups(tt.claims))
		})
	}
}

// TestLookupClaim exercises the dotted-path resolver directly.
func TestLookupClaim(t *testing.T) {
	claims := jwt.MapClaims{
		"top": "v",
		"nested": map[string]interface{}{
			"leaf": "deep",
		},
	}

	v, ok := lookupClaim(claims, "top")
	assert.True(t, ok)
	assert.Equal(t, "v", v)

	v, ok = lookupClaim(claims, "nested.leaf")
	assert.True(t, ok)
	assert.Equal(t, "deep", v)

	_, ok = lookupClaim(claims, "nested.missing")
	assert.False(t, ok)

	_, ok = lookupClaim(claims, "top.too.deep")
	assert.False(t, ok)
}
