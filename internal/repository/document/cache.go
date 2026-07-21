package document

import (
	"context"
	"encoding/json"
	"time"

	"github.com/tararahuuw/ragsytem/internal/infra/cache"
	"github.com/tararahuuw/ragsytem/internal/logger"
	uploadmodel "github.com/tararahuuw/ragsytem/internal/model/upload"
)

// cachedRepository decorates the document Repository with cache-aside reads.
// Completed documents are effectively immutable (there is no update/delete
// endpoint) and read-heavy, so caching their metadata is safe and useful.
//
// Note: only DB metadata is cached here. The presigned download URL is generated
// fresh per response in the service layer, so it never goes stale in the cache.
//
// Invalidation of the LIST keys happens in the upload service when a new document
// completes (see internal/service/upload) — this repo only reads.
//
// Fail-open: cache errors are logged and fall through to the inner (DB) repo.
type cachedRepository struct {
	inner Repository
	cache cache.Cache
	ttl   time.Duration
}

// NewCachedRepository wraps a document Repository with a cache layer.
func NewCachedRepository(inner Repository, c cache.Cache, ttl time.Duration) Repository {
	return &cachedRepository{inner: inner, cache: c, ttl: ttl}
}

func (r *cachedRepository) List(ctx context.Context, orgCode string) ([]uploadmodel.UploadLog, error) {
	key := cache.KeyDocList(orgCode)
	if b, ok, err := r.cache.Get(ctx, key); err != nil {
		logger.FromContext(ctx).Warn("doc cache: get failed (fail-open)", "key", key, "error", err)
	} else if ok {
		var docs []uploadmodel.UploadLog
		if json.Unmarshal(b, &docs) == nil {
			return docs, nil
		}
	}

	docs, err := r.inner.List(ctx, orgCode)
	if err != nil {
		return nil, err
	}
	if b, err := json.Marshal(docs); err == nil {
		if err := r.cache.Set(ctx, key, b, r.ttl); err != nil {
			logger.FromContext(ctx).Warn("doc cache: set failed (fail-open)", "key", key, "error", err)
		}
	}
	return docs, nil
}

func (r *cachedRepository) FindByID(ctx context.Context, id uint) (*uploadmodel.UploadLog, error) {
	key := cache.KeyDocByID(id)
	if b, ok, err := r.cache.Get(ctx, key); err != nil {
		logger.FromContext(ctx).Warn("doc cache: get failed (fail-open)", "key", key, "error", err)
	} else if ok {
		var doc uploadmodel.UploadLog
		if json.Unmarshal(b, &doc) == nil {
			return &doc, nil
		}
	}

	doc, err := r.inner.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	// Only cache positive hits: a missing id won't later resurface as that same
	// id, so negative caching adds no value.
	if doc != nil {
		if b, err := json.Marshal(doc); err == nil {
			if err := r.cache.Set(ctx, key, b, r.ttl); err != nil {
				logger.FromContext(ctx).Warn("doc cache: set failed (fail-open)", "key", key, "error", err)
			}
		}
	}
	return doc, nil
}
