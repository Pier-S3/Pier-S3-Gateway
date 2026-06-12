package webui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// statsRequest builds an authorized stats request routed through a mux so the
// {bucket} path value is populated.
func statsRequest(t *testing.T, api *API, bucket string, groups []string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/buckets/{bucket}/stats", http.HandlerFunc(api.HandleBucketStats))
	req := httptest.NewRequest("GET", "/api/v1/buckets/"+bucket+"/stats", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, withGroups(req, groups))
	return w
}

func decodeStats(t *testing.T, w *httptest.ResponseRecorder) BucketStatsResponse {
	t.Helper()
	var resp BucketStatsResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	return resp
}

func TestHandleBucketStatsSumsAllPages(t *testing.T) {
	calls := 0
	backend := &mockBackend{
		listFn: func(_ context.Context, _, prefix, delimiter, token string, _ int32) (*s3.ListObjectsV2Output, error) {
			calls++
			// The walk must traverse every key: no delimiter, no prefix.
			assert.Empty(t, prefix)
			assert.Empty(t, delimiter)
			if token == "" {
				return &s3.ListObjectsV2Output{
					Contents: []types.Object{
						{Key: aws.String("a"), Size: aws.Int64(100)},
						{Key: aws.String("b"), Size: aws.Int64(200)},
					},
					IsTruncated:           aws.Bool(true),
					NextContinuationToken: aws.String("page2"),
				}, nil
			}
			return &s3.ListObjectsV2Output{
				Contents:    []types.Object{{Key: aws.String("c"), Size: aws.Int64(50)}},
				IsTruncated: aws.Bool(false),
			}, nil
		},
	}
	api := newAPIWithBackend(backend, testLogger())

	w := statsRequest(t, api, "data", []string{"data-ro"})
	require.Equal(t, http.StatusOK, w.Code)
	resp := decodeStats(t, w)
	assert.Equal(t, int64(3), resp.ObjectCount)
	assert.Equal(t, int64(350), resp.SizeBytes)
	assert.False(t, resp.Truncated)
	assert.Equal(t, 2, calls)
}

func TestHandleBucketStatsCaches(t *testing.T) {
	calls := 0
	backend := &mockBackend{
		listFn: func(_ context.Context, _, _, _, _ string, _ int32) (*s3.ListObjectsV2Output, error) {
			calls++
			return &s3.ListObjectsV2Output{
				Contents: []types.Object{{Key: aws.String("a"), Size: aws.Int64(10)}},
			}, nil
		},
	}
	api := newAPIWithBackend(backend, testLogger())

	w1 := statsRequest(t, api, "data", []string{"data-ro"})
	require.Equal(t, http.StatusOK, w1.Code)
	w2 := statsRequest(t, api, "data", []string{"data-ro"})
	require.Equal(t, http.StatusOK, w2.Code)

	assert.Equal(t, 1, calls, "second request within the TTL must hit the cache")
	assert.Equal(t, decodeStats(t, w1), decodeStats(t, w2))
}

func TestHandleBucketStatsTruncatesAtPageCap(t *testing.T) {
	backend := &mockBackend{
		listFn: func(_ context.Context, _, _, _, _ string, _ int32) (*s3.ListObjectsV2Output, error) {
			// Always claim there is more: the walk must stop at the cap.
			return &s3.ListObjectsV2Output{
				Contents:              []types.Object{{Key: aws.String("k"), Size: aws.Int64(1)}},
				IsTruncated:           aws.Bool(true),
				NextContinuationToken: aws.String("more"),
			}, nil
		},
	}
	api := newAPIWithBackend(backend, testLogger())

	w := statsRequest(t, api, "huge", []string{"huge-ro"})
	require.Equal(t, http.StatusOK, w.Code)
	resp := decodeStats(t, w)
	assert.True(t, resp.Truncated)
	assert.Equal(t, int64(statsMaxPages), resp.ObjectCount)
}

func TestHandleBucketStatsACL(t *testing.T) {
	api := newAPIWithBackend(&mockBackend{}, testLogger())

	// No grant on the bucket at all.
	w := statsRequest(t, api, "secret", []string{"other-rw"})
	assert.Equal(t, http.StatusForbidden, w.Code)

	// Write-only must not leak listing-derived totals either.
	w = statsRequest(t, api, "drop", []string{"drop-wo"})
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestHandleBucketStatsQuota(t *testing.T) {
	backend := &mockBackend{
		listFn: func(_ context.Context, _, _, _, _ string, _ int32) (*s3.ListObjectsV2Output, error) {
			return &s3.ListObjectsV2Output{}, nil
		},
	}
	api := newAPIWithBackend(backend, testLogger())
	api.SetBucketQuotas(map[string]int64{"data": 1 << 30, "*": 1 << 20})

	resp := decodeStats(t, statsRequest(t, api, "data", []string{"data-ro"}))
	assert.Equal(t, int64(1<<30), resp.QuotaBytes, "exact name wins")

	resp = decodeStats(t, statsRequest(t, api, "misc", []string{"misc-ro"}))
	assert.Equal(t, int64(1<<20), resp.QuotaBytes, "wildcard default applies")
}
