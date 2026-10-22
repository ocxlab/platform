package media

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Storage keeps internal S3 traffic separate from client-reachable signed URLs.
// Neither endpoint is accepted from an application request.
type Storage struct {
	*Signer
	client *s3.Client
	bucket string
	slots  chan struct{}
}

func FromEnv(getenv func(string) string) (*Storage, error) {
	if getenv("OCP_MEDIA_ENABLED") != "true" {
		return nil, nil
	}
	internal, external := getenv("OCP_MEDIA_INTERNAL_ENDPOINT"), getenv("OCP_MEDIA_PUBLIC_ENDPOINT")
	region, bucket := getenv("OCP_MEDIA_REGION"), getenv("OCP_MEDIA_BUCKET")
	access, secret := getenv("OCP_MEDIA_ACCESS_KEY"), getenv("OCP_MEDIA_SECRET_KEY")
	if access == "" || secret == "" {
		return nil, errors.New("missing media credentials")
	}
	creds := credentials.NewStaticCredentialsProvider(access, secret, "")
	signer, err := NewSigner(external, region, bucket, creds)
	if err != nil {
		return nil, err
	}
	// Apply the same endpoint/key configuration validation before constructing
	// the internal client, rather than allowing an accidental default AWS target.
	if _, err = NewSigner(internal, region, bucket, creds); err != nil {
		return nil, err
	}
	client := s3.NewFromConfig(aws.Config{Region: region, Credentials: creds, RetryMaxAttempts: 2, RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired}, func(o *s3.Options) { o.BaseEndpoint = aws.String(internal); o.UsePathStyle = true })
	return &Storage{Signer: signer, client: client, bucket: bucket, slots: make(chan struct{}, 2)}, nil
}

// Process reads the original from storage, never from an API upload body.
// A deterministic derivative key makes a retry after SQL failure harmless.
func (s *Storage) Process(ctx context.Context, original, final, mediaType, checksum string, size int64) (int, int, error) {
	select {
	case s.slots <- struct{}{}:
		defer func() { <-s.slots }()
	case <-ctx.Done():
		return 0, 0, ctx.Err()
	}
	if !validKey(original) || !validKey(final) || original == final {
		return 0, 0, ErrInvalid
	}
	obj, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(original)})
	if err != nil {
		return 0, 0, err
	}
	defer obj.Body.Close()
	if obj.ContentLength == nil || *obj.ContentLength != size || obj.ContentType == nil || *obj.ContentType != mediaType {
		return 0, 0, ErrInvalid
	}
	raw, err := io.ReadAll(io.LimitReader(obj.Body, MaxUploadBytes+1))
	if err != nil {
		return 0, 0, err
	}
	data, w, h, err := Derivative(raw, mediaType, checksum, size)
	if err != nil {
		return 0, 0, err
	}
	sum := sha256.Sum256(data)
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(final), Body: bytes.NewReader(data), ContentLength: aws.Int64(int64(len(data))), ContentType: aws.String("image/jpeg"), ChecksumSHA256: aws.String(base64.StdEncoding.EncodeToString(sum[:]))})
	return w, h, err
}

const GrantLifetime = 5 * time.Minute
