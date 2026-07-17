// Package minio wraps the MinIO Go SDK with the few operations the upload flow
// needs: streaming put, server-side compose (merge), list, remove, stat, and
// presigned GET URLs. Mirrors elArch's MinioUtils.
package minio

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/minio/minio-go/v7/pkg/lifecycle"

	"github.com/tararahuuw/ragsytem/internal/config"
)

// Client is a thin wrapper around *minio.Client bound to one bucket.
type Client struct {
	raw    *minio.Client
	bucket string
}

// New builds a MinIO client and ensures the target bucket exists.
func New(cfg *config.Config) (*Client, error) {
	raw, err := minio.New(cfg.MinioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.MinioAccessKey, cfg.MinioSecretKey, ""),
		Secure: cfg.MinioUseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("minio: new client: %w", err)
	}

	c := &Client{raw: raw, bucket: cfg.MinioBucket}
	if err := c.ensureBucket(context.Background()); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Client) ensureBucket(ctx context.Context) error {
	exists, err := c.raw.BucketExists(ctx, c.bucket)
	if err != nil {
		return fmt.Errorf("minio: bucket exists check: %w", err)
	}
	if !exists {
		if err := c.raw.MakeBucket(ctx, c.bucket, minio.MakeBucketOptions{}); err != nil {
			return fmt.Errorf("minio: make bucket %q: %w", c.bucket, err)
		}
	}
	// Best-effort: auto-expire abandoned temp chunks so storage doesn't grow
	// unbounded when uploads are never completed. Non-fatal if unsupported.
	if err := c.setTempChunkLifecycle(ctx); err != nil {
		slog.Warn("minio: could not set temp-chunk lifecycle (abandoned chunks won't auto-expire)", "error", err)
	}
	return nil
}

func (c *Client) setTempChunkLifecycle(ctx context.Context) error {
	cfg := lifecycle.NewConfiguration()
	cfg.Rules = []lifecycle.Rule{{
		ID:         "expire-temp-chunks",
		Status:     "Enabled",
		RuleFilter: lifecycle.Filter{Prefix: "temp_chunks/"},
		Expiration: lifecycle.Expiration{Days: 1},
	}}
	return c.raw.SetBucketLifecycle(ctx, c.bucket, cfg)
}

// Bucket returns the bound bucket name.
func (c *Client) Bucket() string { return c.bucket }

// Put streams an object of a known size into the bucket.
func (c *Client) Put(ctx context.Context, object string, r io.Reader, size int64, contentType string) error {
	_, err := c.raw.PutObject(ctx, c.bucket, object, r, size, minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return fmt.Errorf("minio: put %q: %w", object, err)
	}
	return nil
}

// Compose merges the given source objects (in order) into one destination
// object, server-side (no download/re-upload).
func (c *Client) Compose(ctx context.Context, destObject string, srcObjects []string) error {
	dst := minio.CopyDestOptions{Bucket: c.bucket, Object: destObject}
	srcs := make([]minio.CopySrcOptions, 0, len(srcObjects))
	for _, s := range srcObjects {
		srcs = append(srcs, minio.CopySrcOptions{Bucket: c.bucket, Object: s})
	}
	if _, err := c.raw.ComposeObject(ctx, dst, srcs...); err != nil {
		return fmt.Errorf("minio: compose %q: %w", destObject, err)
	}
	return nil
}

// Exists reports whether an object is present.
func (c *Client) Exists(ctx context.Context, object string) (bool, error) {
	_, err := c.raw.StatObject(ctx, c.bucket, object, minio.StatObjectOptions{})
	if err != nil {
		resp := minio.ToErrorResponse(err)
		if resp.Code == "NoSuchKey" || resp.StatusCode == 404 {
			return false, nil
		}
		return false, fmt.Errorf("minio: stat %q: %w", object, err)
	}
	return true, nil
}

// List returns object names under a prefix.
func (c *Client) List(ctx context.Context, prefix string) ([]string, error) {
	var names []string
	for obj := range c.raw.ListObjects(ctx, c.bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
		if obj.Err != nil {
			return nil, fmt.Errorf("minio: list %q: %w", prefix, obj.Err)
		}
		names = append(names, obj.Key)
	}
	return names, nil
}

// Remove deletes the given objects (best-effort; returns first error).
func (c *Client) Remove(ctx context.Context, objects []string) error {
	for _, o := range objects {
		if err := c.raw.RemoveObject(ctx, c.bucket, o, minio.RemoveObjectOptions{}); err != nil {
			return fmt.Errorf("minio: remove %q: %w", o, err)
		}
	}
	return nil
}

// PresignedGetURL returns a time-limited download URL for an object. If
// downloadName is non-empty, the URL forces a download with that filename.
func (c *Client) PresignedGetURL(ctx context.Context, object string, expiry time.Duration, downloadName string) (string, error) {
	params := url.Values{}
	if downloadName != "" {
		params.Set("response-content-disposition", fmt.Sprintf(`attachment; filename="%s"`, downloadName))
	}
	u, err := c.raw.PresignedGetObject(ctx, c.bucket, object, expiry, params)
	if err != nil {
		return "", fmt.Errorf("minio: presign %q: %w", object, err)
	}
	return u.String(), nil
}
