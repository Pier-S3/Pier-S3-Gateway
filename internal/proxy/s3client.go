package proxy

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Client wraps the AWS S3 client configured for a custom endpoint (SeaweedFS).
//
// In addition to the high-level SDK client it retains the raw connection
// parameters (endpoint, region, credentials provider) so that the streaming
// pass-through handler can re-sign forwarded HTTP requests with SigV4 against
// the same backend.
type S3Client struct {
	Client *s3.Client

	endpoint string
	region   string
	creds    aws.CredentialsProvider
}

// NewS3Client creates a new S3 client pointing to the given endpoint with static credentials.
func NewS3Client(endpoint, accessKey, secretKey, region string) (*S3Client, error) {
	if endpoint == "" || accessKey == "" || secretKey == "" {
		return nil, fmt.Errorf("S3 endpoint, access key, and secret key are required")
	}

	// Region is required by the SigV4 signer; default to us-east-1 when callers
	// leave it blank so that signing never produces an empty scope.
	if region == "" {
		region = "us-east-1"
	}

	credsProvider := credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")

	client := s3.New(s3.Options{
		BaseEndpoint: aws.String(endpoint),
		Region:       region,
		Credentials:  credsProvider,
		UsePathStyle: true, // Required for SeaweedFS / MinIO
	})

	return &S3Client{
		Client:   client,
		endpoint: strings.TrimRight(endpoint, "/"),
		region:   region,
		creds:    credsProvider,
	}, nil
}

// Endpoint returns the configured S3 backend endpoint (trailing slash trimmed).
func (c *S3Client) Endpoint() string { return c.endpoint }

// Region returns the configured signing region.
func (c *S3Client) Region() string { return c.region }

// Credentials retrieves the static service credentials used for signing.
func (c *S3Client) Credentials(ctx context.Context) (aws.Credentials, error) {
	if c.creds == nil {
		return aws.Credentials{}, fmt.Errorf("s3 client has no credentials provider configured")
	}
	return c.creds.Retrieve(ctx)
}

// ListBuckets lists all buckets from the S3 backend.
func (c *S3Client) ListBuckets(ctx context.Context) (*s3.ListBucketsOutput, error) {
	return c.Client.ListBuckets(ctx, &s3.ListBucketsInput{})
}

// ListObjects lists objects in a bucket with optional prefix and delimiter.
func (c *S3Client) ListObjects(ctx context.Context, bucket, prefix, delimiter, continuationToken string, maxKeys int32) (*s3.ListObjectsV2Output, error) {
	input := &s3.ListObjectsV2Input{
		Bucket:    aws.String(bucket),
		Prefix:    aws.String(prefix),
		Delimiter: aws.String(delimiter),
		MaxKeys:   aws.Int32(maxKeys),
	}
	if continuationToken != "" {
		input.ContinuationToken = aws.String(continuationToken)
	}
	return c.Client.ListObjectsV2(ctx, input)
}

// HeadObject gets metadata for an object.
func (c *S3Client) HeadObject(ctx context.Context, bucket, key string) (*s3.HeadObjectOutput, error) {
	return c.Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
}

// DeleteObject deletes an object from a bucket.
func (c *S3Client) DeleteObject(ctx context.Context, bucket, key string) (*s3.DeleteObjectOutput, error) {
	return c.Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
}
