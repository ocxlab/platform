// Package media provides object-store signing, not merchant authorization or
// publication. Callers must authorize a server-assigned key before signing it.
package media

import (
	"context"
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const MaxUploadBytes int64 = 10 * 1024 * 1024

type Signer struct {
	client *s3.PresignClient
	bucket string
}

// NewSigner signs the client-reachable endpoint directly. Rewriting its host
// after signing invalidates the signature. No request to storage is needed.
func NewSigner(endpoint, region, bucket string, credentials aws.CredentialsProvider) (*Signer, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") ||
		u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") ||
		region == "" || bucket == "" || strings.ContainsAny(bucket, "/\\ ") || credentials == nil {
		return nil, errors.New("invalid object-store signing configuration")
	}
	client := s3.NewFromConfig(aws.Config{Region: region, Credentials: credentials,
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired}, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(strings.TrimSuffix(endpoint, "/"))
		o.UsePathStyle = true
	})
	return &Signer{client: s3.NewPresignClient(client), bucket: bucket}, nil
}

func validKey(key string) bool {
	if len(key) == 0 || len(key) > 512 || strings.HasPrefix(key, "/") {
		return false
	}
	for _, segment := range strings.Split(key, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
		for _, ch := range segment {
			if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '-' || ch == '_' || ch == '.') {
				return false
			}
		}
	}
	return true
}

// Upload binds a short-lived PUT to one key, media type, length, and checksum.
// The caller must return SignedHeader along with URL; clients must echo them.
// These grants are bearer capabilities and reusable until expiry, not single-use.
func (s *Signer) Upload(ctx context.Context, key, mediaType, checksum string, size int64, ttl time.Duration) (*v4.PresignedHTTPRequest, error) {
	sum, err := base64.StdEncoding.DecodeString(checksum)
	if !validKey(key) || err != nil || len(sum) != 32 || size < 1 || size > MaxUploadBytes ||
		ttl < time.Second || ttl > 15*time.Minute ||
		(mediaType != "image/jpeg" && mediaType != "image/png" && mediaType != "image/webp") {
		return nil, errors.New("invalid media upload request")
	}
	return s.client.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket), Key: aws.String(key), ContentType: aws.String(mediaType),
		ContentLength: aws.Int64(size), ChecksumSHA256: aws.String(checksum),
	}, s3.WithPresignExpires(ttl))
}

func (s *Signer) Download(ctx context.Context, key string, ttl time.Duration) (*v4.PresignedHTTPRequest, error) {
	if !validKey(key) || ttl < time.Second || ttl > 15*time.Minute {
		return nil, errors.New("invalid media download request")
	}
	return s.client.PresignGetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)}, s3.WithPresignExpires(ttl))
}
