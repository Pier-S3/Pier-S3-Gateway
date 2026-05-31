package proxy

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Backend is the seam the handler depends on for talking to the object
// store. *S3Client satisfies it. Tests inject a fake implementation so the
// bucket-listing and pass-through paths can be exercised without a live
// SeaweedFS / MinIO endpoint.
type S3Backend interface {
	// ListBuckets returns all buckets known to the backend.
	ListBuckets(ctx context.Context) (*s3.ListBucketsOutput, error)

	// Endpoint returns the backend base URL (no trailing slash).
	Endpoint() string

	// Region returns the SigV4 signing region.
	Region() string

	// Credentials returns the service-account credentials used to sign
	// forwarded requests.
	Credentials(ctx context.Context) (aws.Credentials, error)
}

// Compile-time assertion that the concrete client satisfies the seam.
var _ S3Backend = (*S3Client)(nil)
