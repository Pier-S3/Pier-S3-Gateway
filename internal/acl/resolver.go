package acl

import (
	"net/url"
	"sort"
	"strings"
)

// BucketPermission represents a user's access to a bucket.
type BucketPermission struct {
	Name   string `json:"name"`
	Read   bool   `json:"read"`
	Write  bool   `json:"write"`
	Delete bool   `json:"delete"`
}

// WildcardBucket is the bucket token that grants a policy across ALL buckets,
// e.g. the group "*-ro" grants read on every bucket. It is matched literally
// as the bucket part of a group name.
const WildcardBucket = "*"

// policyGrants describes the bucket operations a policy allows. Read controls
// GET/HEAD/OPTIONS; write controls PUT/POST; delete controls DELETE. Bucket and
// object administration is never granted here - it is always blocked by
// IsAdminOperation regardless of policy.
type policyGrants struct {
	read   bool
	write  bool
	delete bool
}

// validPolicies maps each recognized policy suffix to the operations it grants.
//
//	ro - read only
//	rw - full read + write (write implies delete, like a Unix file you can write)
//	wo - write only (upload): create/overwrite objects, but cannot read or delete
var validPolicies = map[string]policyGrants{
	"ro": {read: true},
	"rw": {read: true, write: true, delete: true},
	"wo": {write: true},
}

// parseGroup parses a Keycloak group name into a bucket name and policy.
// Returns bucket, policy (a key of validPolicies), and ok=true if valid.
// The bucket may be the WildcardBucket "*", which grants the policy on every
// bucket. Strips all leading "/" from the group name (Keycloak emits group
// paths with a leading slash, and nested paths may yield multiple).
// Examples:
//
//	parseGroup("reports-ro")     → ("reports", "ro", true)
//	parseGroup("raw-events-rw")  → ("raw-events", "rw", true)
//	parseGroup("uploads-wo")     → ("uploads", "wo", true)
//	parseGroup("*-ro")           → ("*", "ro", true)
//	parseGroup("/reports-ro")    → ("reports", "ro", true)
//	parseGroup("no-suffix")      → ("", "", false)
//	parseGroup("")               → ("", "", false)
func parseGroup(group string) (bucket, policy string, ok bool) {
	// Strip all leading "/" so a group name like "//reports-ro" yields the
	// bucket "reports" rather than "/reports" (which could never match a real
	// S3 bucket name and would silently break the ACL rule).
	group = strings.TrimLeft(group, "/")

	// Empty group is invalid
	if group == "" {
		return "", "", false
	}

	// Find the last hyphen
	lastHyphenIdx := strings.LastIndex(group, "-")
	if lastHyphenIdx == -1 {
		// No hyphen found
		return "", "", false
	}

	// Extract potential policy from the end and validate it against the known
	// set of policies.
	potentialPolicy := group[lastHyphenIdx+1:]
	if _, valid := validPolicies[potentialPolicy]; !valid {
		return "", "", false
	}

	// Extract bucket name (everything before the last hyphen)
	bucket = group[:lastHyphenIdx]

	// Bucket name must not be empty
	if bucket == "" {
		return "", "", false
	}

	return bucket, potentialPolicy, true
}

// methodGranted reports whether a policy's grants allow the given HTTP method.
//
//	read   → GET, HEAD, OPTIONS
//	write  → PUT, POST
//	delete → DELETE
func methodGranted(g policyGrants, method string) bool {
	switch method {
	case "GET", "HEAD", "OPTIONS":
		return g.read
	case "PUT", "POST":
		return g.write
	case "DELETE":
		return g.delete
	default:
		return false
	}
}

// CheckAccess reports whether the given groups grant access for the HTTP method
// on the bucket. A group's bucket may be the wildcard "*", which applies its
// policy to every bucket. Returns false if no matching group grants the method.
func CheckAccess(groups []string, bucket, method string) bool {
	for _, group := range groups {
		groupBucket, policy, ok := parseGroup(group)
		if !ok {
			continue
		}

		// A group applies if it targets this bucket explicitly or via wildcard.
		if groupBucket != bucket && groupBucket != WildcardBucket {
			continue
		}

		if methodGranted(validPolicies[policy], method) {
			return true
		}
	}

	return false
}

// adminSubResources is the set of S3 sub-resource query parameters that
// configure or administer a bucket (or its objects) beyond plain object
// read/write. Any request carrying one of these is treated as an admin
// operation and blocked regardless of the caller's ro/rw membership, since
// an rw user must not be able to reconfigure versioning, lifecycle, CORS,
// encryption, replication, object-lock, etc.
var adminSubResources = map[string]struct{}{
	"acl":                 {},
	"policy":              {},
	"versioning":          {},
	"tagging":             {},
	"lifecycle":           {},
	"cors":                {},
	"website":             {},
	"notification":        {},
	"encryption":          {},
	"replication":         {},
	"logging":             {},
	"requestPayment":      {},
	"accelerate":          {},
	"object-lock":         {},
	"lock":                {},
	"retention":           {},
	"legal-hold":          {},
	"inventory":           {},
	"analytics":           {},
	"metrics":             {},
	"select":              {},
	"restore":             {},
	"torrent":             {},
	"publicAccessBlock":   {},
	"ownershipControls":   {},
	"intelligent-tiering": {},
}

// IsAdminOperation returns true for S3 admin operations that are always blocked.
// Blocked: CreateBucket, DeleteBucket, and any bucket/object sub-resource
// administration (PutBucketPolicy, PutBucketAcl, versioning, lifecycle, CORS,
// encryption, replication, object-lock, etc.).
//
// Admin operations are detected by:
//   - PUT or DELETE on a path with only a bucket (no object key)
//   - Any request carrying a known admin sub-resource query parameter
//   - Any path that cannot be parsed (fail closed - block rather than forward
//     a request we cannot fully understand to the backend)
func IsAdminOperation(method string, path string) bool {
	u, err := url.Parse(path)
	if err != nil {
		// Fail closed: a path we cannot parse must not be forwarded to the
		// backend. Block it as an admin operation rather than letting the
		// (parse-independent) ACL check wave it through.
		return true
	}

	query := u.Query()

	// Block any known bucket/object administration sub-resource.
	for param := range adminSubResources {
		if query.Has(param) {
			return true
		}
	}

	// Parse the path (remove query string for path analysis)
	pathOnly := u.Path
	if pathOnly == "" {
		pathOnly = "/"
	}

	// Trim leading/trailing slashes for analysis
	pathOnly = strings.Trim(pathOnly, "/")

	// Split path into segments
	parts := strings.Split(pathOnly, "/")

	// If there's only one part (just bucket, no object key), certain methods are admin ops
	if len(parts) == 1 {
		// PUT or DELETE on bucket level (no object key) are admin operations
		if method == "PUT" || method == "DELETE" {
			return true
		}
	}

	return false
}

// apply merges a policy's grants into a BucketPermission (OR-combining, so a
// user holding several groups for one bucket gets the union of their rights).
func (p *BucketPermission) apply(g policyGrants) {
	p.Read = p.Read || g.read
	p.Write = p.Write || g.write
	p.Delete = p.Delete || g.delete
}

// WildcardGrant returns the combined permission granted across ALL buckets by
// any wildcard ("*-<policy>") groups the user holds, and ok=true if the user
// holds at least one wildcard group. The Name is left empty: the concrete
// bucket names are not known from the groups alone (the caller lists them from
// the storage backend and applies this grant to each).
func WildcardGrant(groups []string) (BucketPermission, bool) {
	var perm BucketPermission
	found := false
	for _, group := range groups {
		bucket, policy, ok := parseGroup(group)
		if !ok || bucket != WildcardBucket {
			continue
		}
		perm.apply(validPolicies[policy])
		found = true
	}
	return perm, found
}

// EffectiveBuckets returns the list of explicitly-named buckets a user can
// access, merging the permissions of all matching groups. Wildcard ("*") groups
// are NOT expanded here (the bucket names are unknown without the storage
// backend) - use WildcardGrant for those and combine at the call site.
func EffectiveBuckets(groups []string) []BucketPermission {
	buckets := make(map[string]*BucketPermission)

	for _, group := range groups {
		bucket, policy, ok := parseGroup(group)
		if !ok || bucket == WildcardBucket {
			continue
		}

		if _, exists := buckets[bucket]; !exists {
			buckets[bucket] = &BucketPermission{Name: bucket}
		}
		buckets[bucket].apply(validPolicies[policy])
	}

	// Convert map to a slice sorted by bucket name for deterministic output.
	// Callers (e.g. the web UI/API) render this list directly, so a stable
	// ordering avoids spurious diffs and flaky behaviour.
	result := make([]BucketPermission, 0, len(buckets))
	for _, perm := range buckets {
		result = append(result, *perm)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result
}
