package webui

import (
	"net/http"
	"sync"
	"time"

	"pier-s3/internal/acl"

	"github.com/aws/aws-sdk-go-v2/aws"
)

// Bucket statistics (size / object count) are computed by walking the bucket
// listing, because plain S3 has no cheap "bucket size" call. Walking is
// bounded and cached so the endpoint cannot be used to hammer the backend:
//
//   - at most statsMaxPages pages are listed per computation; larger buckets
//     return truncated=true and the totals are a lower bound,
//   - results are cached per bucket for statsCacheTTL; concurrent and repeat
//     requests within the window are served from memory.
const (
	// statsMaxPages bounds the listing walk: statsMaxPages * statsPageSize is
	// the maximum number of objects counted per refresh (50k by default).
	statsMaxPages = 50
	statsPageSize = int32(1000)
	statsCacheTTL = 5 * time.Minute
)

// BucketStatsResponse is the response for GET /api/v1/buckets/{bucket}/stats.
type BucketStatsResponse struct {
	Bucket      string `json:"bucket"`
	ObjectCount int64  `json:"object_count"`
	SizeBytes   int64  `json:"size_bytes"`
	// Truncated reports that the bucket has more objects than the walk cap;
	// ObjectCount/SizeBytes are then a lower bound, not the exact total.
	Truncated bool `json:"truncated"`
	// QuotaBytes is the configured quota for this bucket (BUCKET_QUOTAS); 0 /
	// omitted when no quota is configured.
	QuotaBytes int64 `json:"quota_bytes,omitempty"`
}

// statsEntry is a cached computation result.
type statsEntry struct {
	resp     BucketStatsResponse
	fetched  time.Time
	computed bool
}

// statsCache is a small TTL cache guarded by a mutex. A per-bucket in-flight
// flag prevents a thundering herd from launching duplicate walks: followers
// get 202-style "come back" semantics via the cached previous value when one
// exists, or wait out the walk via the in-flight channel when it does not.
type statsCache struct {
	mu      sync.Mutex
	entries map[string]*statsEntry
	pending map[string]chan struct{}
}

func newStatsCache() *statsCache {
	return &statsCache{
		entries: make(map[string]*statsEntry),
		pending: make(map[string]chan struct{}),
	}
}

// HandleBucketStats handles GET /api/v1/buckets/{bucket}/stats.
//
// Authorization mirrors the listing endpoint: the caller needs read access to
// the bucket. Totals come from the bounded, cached walk described above.
func (a *API) HandleBucketStats(w http.ResponseWriter, r *http.Request) {
	bucket := bucketFromRequest(r)
	if bucket == "" {
		writeError(w, http.StatusBadRequest, "bad_request", "bucket name required")
		return
	}

	groups := GetGroups(r)
	if !acl.CheckAccess(groups, bucket, "GET") {
		writeError(w, http.StatusForbidden, "forbidden", "no read access to bucket")
		return
	}

	if !a.backendReady(w) {
		return
	}

	// Fast path: fresh cache entry.
	a.stats.mu.Lock()
	if e, ok := a.stats.entries[bucket]; ok && e.computed && time.Since(e.fetched) < statsCacheTTL {
		resp := e.resp
		a.stats.mu.Unlock()
		writeJSON(w, http.StatusOK, resp)
		return
	}

	// If another request is already walking this bucket, wait for it rather
	// than starting a duplicate walk.
	if ch, inFlight := a.stats.pending[bucket]; inFlight {
		a.stats.mu.Unlock()
		select {
		case <-ch:
		case <-r.Context().Done():
			writeError(w, http.StatusServiceUnavailable, "canceled", "request canceled")
			return
		}
		a.stats.mu.Lock()
		if e, ok := a.stats.entries[bucket]; ok && e.computed {
			resp := e.resp
			a.stats.mu.Unlock()
			writeJSON(w, http.StatusOK, resp)
			return
		}
		a.stats.mu.Unlock()
		writeError(w, http.StatusBadGateway, "upstream_error", "failed to compute bucket stats")
		return
	}

	ch := make(chan struct{})
	a.stats.pending[bucket] = ch
	a.stats.mu.Unlock()

	resp, err := a.computeBucketStats(r, bucket)

	a.stats.mu.Lock()
	delete(a.stats.pending, bucket)
	close(ch)
	if err == nil {
		a.stats.entries[bucket] = &statsEntry{resp: resp, fetched: time.Now(), computed: true}
	}
	a.stats.mu.Unlock()

	if err != nil {
		a.logger.Error("failed to compute bucket stats", "bucket", bucket, "error", err.Error())
		writeError(w, http.StatusBadGateway, "upstream_error", "failed to compute bucket stats")
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// computeBucketStats walks the bucket listing (no delimiter, so every key is
// visited) summing sizes, up to the statsMaxPages cap.
func (a *API) computeBucketStats(r *http.Request, bucket string) (BucketStatsResponse, error) {
	resp := BucketStatsResponse{Bucket: bucket, QuotaBytes: a.quotaFor(bucket)}

	token := ""
	for page := 0; page < statsMaxPages; page++ {
		out, err := a.backend.ListObjects(r.Context(), bucket, "", "", token, statsPageSize)
		if err != nil {
			return BucketStatsResponse{}, err
		}
		for _, obj := range out.Contents {
			resp.ObjectCount++
			resp.SizeBytes += aws.ToInt64(obj.Size)
		}
		if !aws.ToBool(out.IsTruncated) || out.NextContinuationToken == nil {
			return resp, nil
		}
		token = *out.NextContinuationToken
	}
	resp.Truncated = true
	return resp, nil
}

// quotaFor resolves the configured quota for a bucket: an exact name match
// wins, then the "*" default; 0 means "no quota configured".
func (a *API) quotaFor(bucket string) int64 {
	if a.quotas == nil {
		return 0
	}
	if q, ok := a.quotas[bucket]; ok {
		return q
	}
	return a.quotas["*"]
}
