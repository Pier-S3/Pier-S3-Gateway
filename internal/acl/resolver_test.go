package acl

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestParseGroup tests the parseGroup function with various inputs.
func TestParseGroup(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectBucket string
		expectPolicy string
		expectOk     bool
	}{
		// Valid cases
		{
			name:         "simple ro policy",
			input:        "reports-ro",
			expectBucket: "reports",
			expectPolicy: "ro",
			expectOk:     true,
		},
		{
			name:         "simple rw policy",
			input:        "raw-events-rw",
			expectBucket: "raw-events",
			expectPolicy: "rw",
			expectOk:     true,
		},
		{
			name:         "leading slash stripped",
			input:        "/reports-ro",
			expectBucket: "reports",
			expectPolicy: "ro",
			expectOk:     true,
		},
		{
			name:         "multi-hyphen bucket ro",
			input:        "raw-events-data-ro",
			expectBucket: "raw-events-data",
			expectPolicy: "ro",
			expectOk:     true,
		},
		{
			name:         "multi-hyphen bucket rw",
			input:        "raw-events-data-rw",
			expectBucket: "raw-events-data",
			expectPolicy: "rw",
			expectOk:     true,
		},

		// Invalid cases
		{
			name:         "no hyphen",
			input:        "just-a-name",
			expectBucket: "",
			expectPolicy: "",
			expectOk:     false,
		},
		{
			name:         "invalid suffix not policy",
			input:        "no-suffix",
			expectBucket: "",
			expectPolicy: "",
			expectOk:     false,
		},
		{
			name:         "invalid suffix xx",
			input:        "bad-xx",
			expectBucket: "",
			expectPolicy: "",
			expectOk:     false,
		},
		{
			name:         "empty string",
			input:        "",
			expectBucket: "",
			expectPolicy: "",
			expectOk:     false,
		},
		{
			name:         "only ro no bucket",
			input:        "ro",
			expectBucket: "",
			expectPolicy: "",
			expectOk:     false,
		},
		{
			name:         "only rw no bucket",
			input:        "rw",
			expectBucket: "",
			expectPolicy: "",
			expectOk:     false,
		},
		{
			name:         "empty bucket with ro",
			input:        "-ro",
			expectBucket: "",
			expectPolicy: "",
			expectOk:     false,
		},
		{
			name:         "empty bucket with rw",
			input:        "-rw",
			expectBucket: "",
			expectPolicy: "",
			expectOk:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bucket, policy, ok := parseGroup(tt.input)
			assert.Equal(t, tt.expectBucket, bucket, "bucket mismatch")
			assert.Equal(t, tt.expectPolicy, policy, "policy mismatch")
			assert.Equal(t, tt.expectOk, ok, "ok flag mismatch")
		})
	}
}

// TestCheckAccess tests the CheckAccess function with various group and method combinations.
func TestCheckAccess(t *testing.T) {
	tests := []struct {
		name     string
		groups   []string
		bucket   string
		method   string
		expectOk bool
	}{
		// Read-only access
		{
			name:     "ro GET allowed",
			groups:   []string{"reports-ro"},
			bucket:   "reports",
			method:   "GET",
			expectOk: true,
		},
		{
			name:     "ro HEAD allowed",
			groups:   []string{"reports-ro"},
			bucket:   "reports",
			method:   "HEAD",
			expectOk: true,
		},
		{
			name:     "ro OPTIONS allowed",
			groups:   []string{"reports-ro"},
			bucket:   "reports",
			method:   "OPTIONS",
			expectOk: true,
		},
		{
			name:     "ro PUT denied",
			groups:   []string{"reports-ro"},
			bucket:   "reports",
			method:   "PUT",
			expectOk: false,
		},
		{
			name:     "ro DELETE denied",
			groups:   []string{"reports-ro"},
			bucket:   "reports",
			method:   "DELETE",
			expectOk: false,
		},

		// Read-write access
		{
			name:     "rw GET allowed",
			groups:   []string{"reports-rw"},
			bucket:   "reports",
			method:   "GET",
			expectOk: true,
		},
		{
			name:     "rw HEAD allowed",
			groups:   []string{"reports-rw"},
			bucket:   "reports",
			method:   "HEAD",
			expectOk: true,
		},
		{
			name:     "rw OPTIONS allowed",
			groups:   []string{"reports-rw"},
			bucket:   "reports",
			method:   "OPTIONS",
			expectOk: true,
		},
		{
			name:     "rw PUT allowed",
			groups:   []string{"reports-rw"},
			bucket:   "reports",
			method:   "PUT",
			expectOk: true,
		},
		{
			name:     "rw POST allowed",
			groups:   []string{"reports-rw"},
			bucket:   "reports",
			method:   "POST",
			expectOk: true,
		},
		{
			name:     "rw DELETE allowed",
			groups:   []string{"reports-rw"},
			bucket:   "reports",
			method:   "DELETE",
			expectOk: true,
		},

		// No access
		{
			name:     "empty groups no access",
			groups:   []string{},
			bucket:   "reports",
			method:   "GET",
			expectOk: false,
		},
		{
			name:     "wrong bucket",
			groups:   []string{"other-ro"},
			bucket:   "reports",
			method:   "GET",
			expectOk: false,
		},

		// Multiple groups
		{
			name:     "multiple groups first matches",
			groups:   []string{"reports-ro", "dev-rw"},
			bucket:   "reports",
			method:   "GET",
			expectOk: true,
		},
		{
			name:     "multiple groups second matches",
			groups:   []string{"reports-ro", "dev-rw"},
			bucket:   "dev",
			method:   "PUT",
			expectOk: true,
		},
		{
			name:     "multiple groups no match",
			groups:   []string{"reports-ro", "dev-rw"},
			bucket:   "other",
			method:   "GET",
			expectOk: false,
		},

		// Invalid groups (should be ignored)
		{
			name:     "invalid group in list",
			groups:   []string{"invalid", "reports-ro"},
			bucket:   "reports",
			method:   "GET",
			expectOk: true,
		},
		{
			name:     "all invalid groups",
			groups:   []string{"invalid1", "invalid2"},
			bucket:   "reports",
			method:   "GET",
			expectOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CheckAccess(tt.groups, tt.bucket, tt.method)
			assert.Equal(t, tt.expectOk, result, "access check mismatch")
		})
	}
}

// TestIsAdminOperation tests the IsAdminOperation function.
func TestIsAdminOperation(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		path        string
		expectAdmin bool
	}{
		// Query parameter checks
		{
			name:        "PUT with policy query",
			method:      "PUT",
			path:        "/bucket?policy=",
			expectAdmin: true,
		},
		{
			name:        "PUT with acl query",
			method:      "PUT",
			path:        "/bucket?acl=",
			expectAdmin: true,
		},
		{
			name:        "GET with policy query",
			method:      "GET",
			path:        "/bucket?policy=",
			expectAdmin: true,
		},
		{
			name:        "DELETE with acl query",
			method:      "DELETE",
			path:        "/bucket?acl=",
			expectAdmin: true,
		},

		// Bucket-level operations
		{
			name:        "PUT on bucket path",
			method:      "PUT",
			path:        "/bucket",
			expectAdmin: true,
		},
		{
			name:        "DELETE on bucket path",
			method:      "DELETE",
			path:        "/bucket",
			expectAdmin: true,
		},

		// Non-admin operations
		{
			name:        "GET on bucket",
			method:      "GET",
			path:        "/bucket",
			expectAdmin: false,
		},
		{
			name:        "HEAD on bucket",
			method:      "HEAD",
			path:        "/bucket",
			expectAdmin: false,
		},
		{
			name:        "OPTIONS on bucket",
			method:      "OPTIONS",
			path:        "/bucket",
			expectAdmin: false,
		},
		{
			name:        "POST on bucket",
			method:      "POST",
			path:        "/bucket",
			expectAdmin: false,
		},

		// Object-level operations
		{
			name:        "PUT on object",
			method:      "PUT",
			path:        "/bucket/object",
			expectAdmin: false,
		},
		{
			name:        "DELETE on object",
			method:      "DELETE",
			path:        "/bucket/object",
			expectAdmin: false,
		},
		{
			name:        "GET on object",
			method:      "GET",
			path:        "/bucket/object",
			expectAdmin: false,
		},
		{
			name:        "PUT on nested object",
			method:      "PUT",
			path:        "/bucket/folder/object",
			expectAdmin: false,
		},

		// Complex paths
		{
			name:        "PUT on object with query",
			method:      "PUT",
			path:        "/bucket/object?metadata=foo",
			expectAdmin: false,
		},

		// Root path
		{
			name:        "GET root path",
			method:      "GET",
			path:        "/",
			expectAdmin: false,
		},

		// Empty path
		{
			name:        "PUT empty path treated as bucket level",
			method:      "PUT",
			path:        "",
			expectAdmin: true,
		},
		{
			name:        "GET empty path not admin",
			method:      "GET",
			path:        "",
			expectAdmin: false,
		},

		// Trailing slash on bucket is still bucket-level
		{
			name:        "DELETE bucket with trailing slash",
			method:      "DELETE",
			path:        "/bucket/",
			expectAdmin: true,
		},

		// acl/policy query on object path is still an admin op
		{
			name:        "PUT object with acl query is admin",
			method:      "PUT",
			path:        "/bucket/object?acl=",
			expectAdmin: true,
		},
		{
			name:        "GET object with policy query is admin",
			method:      "GET",
			path:        "/bucket/object?policy=",
			expectAdmin: true,
		},

		// Unparseable path: control character makes url.Parse fail. We fail
		// CLOSED (treat as admin/blocked) rather than forwarding a request we
		// cannot fully parse to the backend.
		{
			name:        "unparseable path fails closed to admin",
			method:      "PUT",
			path:        "/bucket\x7f\x00?acl=foo",
			expectAdmin: true,
		},

		// Bucket-management sub-resources are admin ops regardless of method.
		{
			name:        "GET bucket versioning is admin",
			method:      "GET",
			path:        "/bucket?versioning",
			expectAdmin: true,
		},
		{
			name:        "PUT bucket lifecycle is admin",
			method:      "PUT",
			path:        "/bucket?lifecycle",
			expectAdmin: true,
		},
		{
			name:        "PUT bucket encryption is admin",
			method:      "PUT",
			path:        "/bucket?encryption",
			expectAdmin: true,
		},
		{
			name:        "PUT object retention is admin",
			method:      "PUT",
			path:        "/bucket/object?retention",
			expectAdmin: true,
		},
		{
			name:        "GET bucket cors is admin",
			method:      "GET",
			path:        "/bucket?cors",
			expectAdmin: true,
		},

		// POST on object level is never a bucket admin op
		{
			name:        "POST on nested object not admin",
			method:      "POST",
			path:        "/bucket/folder/key",
			expectAdmin: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsAdminOperation(tt.method, tt.path)
			assert.Equal(t, tt.expectAdmin, result, "admin operation check mismatch")
		})
	}
}

// TestEffectiveBuckets tests the EffectiveBuckets function.
func TestEffectiveBuckets(t *testing.T) {
	tests := []struct {
		name         string
		groups       []string
		expectResult map[string]BucketPermission
	}{
		{
			name:   "single ro group",
			groups: []string{"reports-ro"},
			expectResult: map[string]BucketPermission{
				"reports": {Name: "reports", Read: true, Write: false},
			},
		},
		{
			name:   "single rw group",
			groups: []string{"reports-rw"},
			expectResult: map[string]BucketPermission{
				"reports": {Name: "reports", Read: true, Write: true, Delete: true},
			},
		},
		{
			name:   "multiple different buckets",
			groups: []string{"reports-ro", "dev-rw"},
			expectResult: map[string]BucketPermission{
				"reports": {Name: "reports", Read: true, Write: false},
				"dev":     {Name: "dev", Read: true, Write: true, Delete: true},
			},
		},
		{
			name:   "ro and rw for same bucket",
			groups: []string{"reports-ro", "reports-rw"},
			expectResult: map[string]BucketPermission{
				"reports": {Name: "reports", Read: true, Write: true, Delete: true},
			},
		},
		{
			name:   "multiple groups same bucket same policy",
			groups: []string{"data-ro", "data-ro"},
			expectResult: map[string]BucketPermission{
				"data": {Name: "data", Read: true, Write: false},
			},
		},
		{
			name:         "empty groups",
			groups:       []string{},
			expectResult: map[string]BucketPermission{},
		},
		{
			name:   "invalid groups ignored",
			groups: []string{"invalid", "reports-ro", "bad-xx"},
			expectResult: map[string]BucketPermission{
				"reports": {Name: "reports", Read: true, Write: false},
			},
		},
		{
			name:   "complex multi-hyphen buckets",
			groups: []string{"raw-events-data-ro", "processed-data-rw"},
			expectResult: map[string]BucketPermission{
				"raw-events-data": {Name: "raw-events-data", Read: true, Write: false},
				"processed-data":  {Name: "processed-data", Read: true, Write: true, Delete: true},
			},
		},
		{
			name:   "write-only (wo) grants write without read or delete",
			groups: []string{"uploads-wo"},
			expectResult: map[string]BucketPermission{
				"uploads": {Name: "uploads", Read: false, Write: true, Delete: false},
			},
		},
		{
			name:   "ro merged with wo yields read+write, no delete",
			groups: []string{"drop-ro", "drop-wo"},
			expectResult: map[string]BucketPermission{
				"drop": {Name: "drop", Read: true, Write: true, Delete: false},
			},
		},
		{
			name:   "wildcard groups are excluded from named buckets",
			groups: []string{"*-ro", "reports-rw"},
			expectResult: map[string]BucketPermission{
				"reports": {Name: "reports", Read: true, Write: true, Delete: true},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EffectiveBuckets(tt.groups)

			// Convert result to map for easier comparison
			resultMap := make(map[string]BucketPermission)
			for _, bp := range result {
				resultMap[bp.Name] = bp
			}

			assert.Equal(t, tt.expectResult, resultMap, "effective buckets mismatch")
		})
	}
}

// TestEffectiveBucketsSorted verifies that EffectiveBuckets returns its result
// sorted by bucket name, deterministically, regardless of input ordering.
// Callers in the web UI/API render this slice directly and rely on stable order.
func TestEffectiveBucketsSorted(t *testing.T) {
	// Intentionally unsorted, with duplicates and invalid entries mixed in.
	groups := []string{
		"zeta-ro",
		"alpha-rw",
		"mike-ro",
		"alpha-ro", // merges with alpha-rw
		"invalid",
		"bravo-rw",
	}

	want := []BucketPermission{
		{Name: "alpha", Read: true, Write: true, Delete: true},
		{Name: "bravo", Read: true, Write: true, Delete: true},
		{Name: "mike", Read: true, Write: false},
		{Name: "zeta", Read: true, Write: false},
	}

	// Run several times to catch non-deterministic map iteration order.
	for i := 0; i < 10; i++ {
		got := EffectiveBuckets(groups)
		assert.Equal(t, want, got, "EffectiveBuckets must be sorted by name (iteration %d)", i)
	}
}

// TestEffectiveBucketsNilGroups ensures nil input yields a non-nil empty slice
// rather than panicking.
func TestEffectiveBucketsNilGroups(t *testing.T) {
	got := EffectiveBuckets(nil)
	assert.NotNil(t, got, "result should be a non-nil slice")
	assert.Len(t, got, 0, "result should be empty for nil input")
}

// TestCheckAccessEdgeCases covers method/group combinations not exercised above.
func TestCheckAccessEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		groups   []string
		bucket   string
		method   string
		expectOk bool
	}{
		{
			name:     "unknown method on rw denied",
			groups:   []string{"reports-rw"},
			bucket:   "reports",
			method:   "PATCH",
			expectOk: false,
		},
		{
			name:     "unknown method on ro denied",
			groups:   []string{"reports-ro"},
			bucket:   "reports",
			method:   "TRACE",
			expectOk: false,
		},
		{
			name:     "empty method denied",
			groups:   []string{"reports-rw"},
			bucket:   "reports",
			method:   "",
			expectOk: false,
		},
		{
			name:     "nil groups denied",
			groups:   nil,
			bucket:   "reports",
			method:   "GET",
			expectOk: false,
		},
		{
			name:     "ro and rw same bucket, write allowed via rw",
			groups:   []string{"reports-ro", "reports-rw"},
			bucket:   "reports",
			method:   "DELETE",
			expectOk: true,
		},
		{
			name:     "method is case sensitive, lowercase denied",
			groups:   []string{"reports-rw"},
			bucket:   "reports",
			method:   "get",
			expectOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CheckAccess(tt.groups, tt.bucket, tt.method)
			assert.Equal(t, tt.expectOk, result, "access check mismatch")
		})
	}
}

// TestParseGroupExtra covers parseGroup inputs not in the main table.
func TestParseGroupExtra(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectBucket string
		expectPolicy string
		expectOk     bool
	}{
		{
			name:         "only leading slash",
			input:        "/",
			expectBucket: "",
			expectPolicy: "",
			expectOk:     false,
		},
		{
			name:         "double leading slash strips all",
			input:        "//reports-ro",
			expectBucket: "reports",
			expectPolicy: "ro",
			expectOk:     true,
		},
		{
			name:         "uppercase policy not matched",
			input:        "reports-RO",
			expectBucket: "",
			expectPolicy: "",
			expectOk:     false,
		},
		{
			name:         "policy substring not matched",
			input:        "reports-row",
			expectBucket: "",
			expectPolicy: "",
			expectOk:     false,
		},
		{
			name:         "trailing hyphen only",
			input:        "reports-",
			expectBucket: "",
			expectPolicy: "",
			expectOk:     false,
		},
		{
			name:         "write-only policy parsed",
			input:        "uploads-wo",
			expectBucket: "uploads",
			expectPolicy: "wo",
			expectOk:     true,
		},
		{
			name:         "wildcard bucket parsed",
			input:        "*-ro",
			expectBucket: "*",
			expectPolicy: "ro",
			expectOk:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bucket, policy, ok := parseGroup(tt.input)
			assert.Equal(t, tt.expectBucket, bucket, "bucket mismatch")
			assert.Equal(t, tt.expectPolicy, policy, "policy mismatch")
			assert.Equal(t, tt.expectOk, ok, "ok flag mismatch")
		})
	}
}

// TestCheckAccessWriteOnly verifies the wo policy: write (PUT/POST) is allowed,
// while read (GET/HEAD) and delete (DELETE) are denied.
func TestCheckAccessWriteOnly(t *testing.T) {
	groups := []string{"uploads-wo"}
	cases := map[string]bool{
		"PUT": true, "POST": true,
		"GET": false, "HEAD": false, "OPTIONS": false, "DELETE": false,
	}
	for method, want := range cases {
		assert.Equalf(t, want, CheckAccess(groups, "uploads", method), "wo %s", method)
	}
	assert.False(t, CheckAccess(groups, "other", "PUT"), "wo must not leak to other buckets")
}

// TestCheckAccessRwDelete confirms rw still allows DELETE (write implies delete).
func TestCheckAccessRwDelete(t *testing.T) {
	assert.True(t, CheckAccess([]string{"data-rw"}, "data", "DELETE"))
	assert.False(t, CheckAccess([]string{"data-ro"}, "data", "DELETE"))
}

// TestCheckAccessWildcard verifies "*-<policy>" applies to every bucket with the
// policy's methods, and nothing beyond them.
func TestCheckAccessWildcard(t *testing.T) {
	assert.True(t, CheckAccess([]string{"*-ro"}, "anything", "GET"))
	assert.True(t, CheckAccess([]string{"*-ro"}, "other", "HEAD"))
	assert.False(t, CheckAccess([]string{"*-ro"}, "anything", "PUT"))
	assert.False(t, CheckAccess([]string{"*-ro"}, "anything", "DELETE"))

	assert.True(t, CheckAccess([]string{"*-rw"}, "anything", "DELETE"))
	assert.True(t, CheckAccess([]string{"*-rw"}, "anything", "PUT"))

	groups := []string{"*-ro", "reports-rw"}
	assert.True(t, CheckAccess(groups, "logs", "GET"), "wildcard read on any bucket")
	assert.False(t, CheckAccess(groups, "logs", "PUT"), "named write must not leak to logs")
	assert.True(t, CheckAccess(groups, "reports", "PUT"), "named write applies to its bucket")
}

// TestWildcardGrant covers the wildcard aggregation helper.
func TestWildcardGrant(t *testing.T) {
	_, ok := WildcardGrant([]string{"reports-rw", "logs-ro"})
	assert.False(t, ok, "no wildcard group present")

	perm, ok := WildcardGrant([]string{"*-ro"})
	assert.True(t, ok)
	assert.Equal(t, BucketPermission{Read: true}, perm)

	perm, ok = WildcardGrant([]string{"*-ro", "*-wo", "reports-rw"})
	assert.True(t, ok)
	assert.Equal(t, BucketPermission{Read: true, Write: true}, perm)
	assert.Empty(t, perm.Name, "wildcard grant carries no concrete bucket name")
}
