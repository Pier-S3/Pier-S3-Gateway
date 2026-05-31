package proxy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewS3ClientValidation(t *testing.T) {
	tests := []struct {
		name      string
		endpoint  string
		accessKey string
		secretKey string
		region    string
		wantErr   bool
	}{
		{"missing endpoint", "", "ak", "sk", "us-east-1", true},
		{"missing access key", "http://x:8333", "", "sk", "us-east-1", true},
		{"missing secret key", "http://x:8333", "ak", "", "us-east-1", true},
		{"valid", "http://x:8333", "ak", "sk", "us-east-1", false},
		{"valid empty region defaults", "http://x:8333", "ak", "sk", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewS3Client(tt.endpoint, tt.accessKey, tt.secretKey, tt.region)
			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, c)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, c)
		})
	}
}

func TestS3ClientAccessors(t *testing.T) {
	c, err := NewS3Client("http://seaweed:8333/", "ak", "sk", "")
	require.NoError(t, err)

	// Trailing slash is trimmed.
	assert.Equal(t, "http://seaweed:8333", c.Endpoint())
	// Empty region defaults to us-east-1.
	assert.Equal(t, "us-east-1", c.Region())

	creds, err := c.Credentials(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "ak", creds.AccessKeyID)
	assert.Equal(t, "sk", creds.SecretAccessKey)
}

func TestS3ClientCredentialsNoProvider(t *testing.T) {
	c := &S3Client{}
	_, err := c.Credentials(context.Background())
	require.Error(t, err)
}

func TestS3ClientSatisfiesBackend(t *testing.T) {
	c, err := NewS3Client("http://x:8333", "ak", "sk", "us-east-1")
	require.NoError(t, err)
	var _ S3Backend = c
}
