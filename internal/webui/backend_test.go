package webui

import (
	"context"
	"testing"
	"time"

	"pier-s3/internal/proxy"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewS3BackendNil(t *testing.T) {
	assert.Nil(t, newS3Backend(nil), "nil client yields nil backend")
}

func TestNewS3BackendWrapsClient(t *testing.T) {
	// A real (unconnected) S3 client is enough to exercise the adapter wiring;
	// we don't make network calls, only verify the adapter is constructed and
	// forwards to the underlying client.
	sc, err := proxy.NewS3Client("https://example.invalid", "ak", "sk", "us-east-1")
	require.NoError(t, err)

	backend := newS3Backend(sc)
	require.NotNil(t, backend)

	adapter, ok := backend.(*s3ClientAdapter)
	require.True(t, ok)
	assert.Same(t, sc, adapter.client)

	// Calls fail against the invalid endpoint, but they must reach the SDK
	// (i.e. return an error rather than panic), proving the forwarding wiring.
	// A short deadline keeps the test fast despite SDK retries.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err = backend.ListObjects(ctx, "b", "", "/", "", 10)
	assert.Error(t, err)
	_, err = backend.GetObject(ctx, "b", "k")
	assert.Error(t, err)
	_, err = backend.HeadObject(ctx, "b", "k")
	assert.Error(t, err)
	_, err = backend.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String("b"), Key: aws.String("k")})
	assert.Error(t, err)
	_, err = backend.DeleteObject(ctx, "b", "k")
	assert.Error(t, err)
}
