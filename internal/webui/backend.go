package webui

import (
	"context"

	"pier-s3/internal/proxy"

	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// s3Backend abstracts the S3 operations the Web UI API needs.
//
// It exists so the API handlers can depend on an interface rather than the
// concrete *proxy.S3Client (which wraps an *s3.Client). This makes the
// handlers unit-testable with a mock and keeps the proxy package unchanged.
type s3Backend interface {
	ListBuckets(ctx context.Context) (*s3.ListBucketsOutput, error)
	ListObjects(ctx context.Context, bucket, prefix, delimiter, continuationToken string, maxKeys int32) (*s3.ListObjectsV2Output, error)
	GetObject(ctx context.Context, bucket, key string) (*s3.GetObjectOutput, error)
	HeadObject(ctx context.Context, bucket, key string) (*s3.HeadObjectOutput, error)
	PutObject(ctx context.Context, input *s3.PutObjectInput) (*s3.PutObjectOutput, error)
	DeleteObject(ctx context.Context, bucket, key string) (*s3.DeleteObjectOutput, error)
}

// s3ClientAdapter adapts *proxy.S3Client to the s3Backend interface.
//
// proxy.S3Client already provides ListObjects, HeadObject and DeleteObject as
// methods; GetObject and PutObject are exposed via its embedded *s3.Client, so
// the adapter forwards those through the underlying client.
type s3ClientAdapter struct {
	client *proxy.S3Client
}

// newS3Backend wraps a *proxy.S3Client as an s3Backend.
//
// A nil client yields a nil backend so callers/handlers can detect an
// unconfigured backend and fail gracefully rather than panic.
func newS3Backend(client *proxy.S3Client) s3Backend {
	if client == nil {
		return nil
	}
	return &s3ClientAdapter{client: client}
}

func (a *s3ClientAdapter) ListBuckets(ctx context.Context) (*s3.ListBucketsOutput, error) {
	return a.client.ListBuckets(ctx)
}

func (a *s3ClientAdapter) ListObjects(ctx context.Context, bucket, prefix, delimiter, continuationToken string, maxKeys int32) (*s3.ListObjectsV2Output, error) {
	return a.client.ListObjects(ctx, bucket, prefix, delimiter, continuationToken, maxKeys)
}

func (a *s3ClientAdapter) GetObject(ctx context.Context, bucket, key string) (*s3.GetObjectOutput, error) {
	return a.client.Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
}

func (a *s3ClientAdapter) HeadObject(ctx context.Context, bucket, key string) (*s3.HeadObjectOutput, error) {
	return a.client.HeadObject(ctx, bucket, key)
}

func (a *s3ClientAdapter) PutObject(ctx context.Context, input *s3.PutObjectInput) (*s3.PutObjectOutput, error) {
	// The Web UI streams a non-seekable HTTP request body straight into
	// PutObject. The default SigV4 flow computes a payload SHA256, which seeks
	// the body and fails with "request stream is not seekable" against
	// SeaweedFS/MinIO. Swap in the unsigned-payload middleware so the body
	// streams without a pre-read; the handler still sets ContentLength to bound
	// the request.
	return a.client.Client.PutObject(ctx, input, func(o *s3.Options) {
		o.APIOptions = append(o.APIOptions, v4.SwapComputePayloadSHA256ForUnsignedPayloadMiddleware)
	})
}

func (a *s3ClientAdapter) DeleteObject(ctx context.Context, bucket, key string) (*s3.DeleteObjectOutput, error) {
	return a.client.DeleteObject(ctx, bucket, key)
}
