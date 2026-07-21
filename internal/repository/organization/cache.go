package organization

import (
	"context"
	"encoding/json"
	"time"

	"github.com/tararahuuw/ragsytem/internal/infra/cache"
	"github.com/tararahuuw/ragsytem/internal/logger"
	orgmodel "github.com/tararahuuw/ragsytem/internal/model/organization"
)

// cachedRepository decorates a Repository with cache-aside reads and write
// invalidation. Organizations are a hot, rarely-changing registry (ExistsActive
// is checked on every register), so they benefit most from caching.
//
// Fail-open: any cache error is logged and the call falls through to the inner
// (DB) repository — correctness never depends on Redis.
type cachedRepository struct {
	inner Repository
	cache cache.Cache
	ttl   time.Duration
}

// NewCachedRepository wraps a Repository with a cache layer. Pass the same
// instance everywhere the org repo is used (auth + organization modules) so
// writes here invalidate reads there via the shared Redis.
func NewCachedRepository(inner Repository, c cache.Cache, ttl time.Duration) Repository {
	return &cachedRepository{inner: inner, cache: c, ttl: ttl}
}

func (r *cachedRepository) ExistsActive(ctx context.Context, code string) (bool, error) {
	key := cache.KeyOrgExists(code)
	if b, ok, err := r.cache.Get(ctx, key); err != nil {
		logger.FromContext(ctx).Warn("org cache: get failed (fail-open)", "key", key, "error", err)
	} else if ok {
		return string(b) == "1", nil
	}

	v, err := r.inner.ExistsActive(ctx, code)
	if err != nil {
		return false, err
	}
	val := []byte("0")
	if v {
		val = []byte("1")
	}
	r.set(ctx, key, val)
	return v, nil
}

func (r *cachedRepository) GetByCode(ctx context.Context, code string) (*orgmodel.Organization, error) {
	key := cache.KeyOrgGet(code)
	if b, ok, err := r.cache.Get(ctx, key); err != nil {
		logger.FromContext(ctx).Warn("org cache: get failed (fail-open)", "key", key, "error", err)
	} else if ok {
		if string(b) == "null" {
			return nil, nil // cached negative lookup
		}
		var o orgmodel.Organization
		if json.Unmarshal(b, &o) == nil {
			return &o, nil
		}
	}

	o, err := r.inner.GetByCode(ctx, code)
	if err != nil {
		return nil, err
	}
	if o == nil {
		r.set(ctx, key, []byte("null"))
	} else if b, err := json.Marshal(o); err == nil {
		r.set(ctx, key, b)
	}
	return o, nil
}

func (r *cachedRepository) List(ctx context.Context) ([]orgmodel.Organization, error) {
	key := cache.KeyOrgList()
	if b, ok, err := r.cache.Get(ctx, key); err != nil {
		logger.FromContext(ctx).Warn("org cache: get failed (fail-open)", "key", key, "error", err)
	} else if ok {
		var orgs []orgmodel.Organization
		if json.Unmarshal(b, &orgs) == nil {
			return orgs, nil
		}
	}

	orgs, err := r.inner.List(ctx)
	if err != nil {
		return nil, err
	}
	if b, err := json.Marshal(orgs); err == nil {
		r.set(ctx, key, b)
	}
	return orgs, nil
}

// --- writes: pass through, then invalidate ---

func (r *cachedRepository) Create(ctx context.Context, o *orgmodel.Organization) error {
	if err := r.inner.Create(ctx, o); err != nil {
		return err
	}
	r.invalidate(ctx, o.Code)
	return nil
}

func (r *cachedRepository) Update(ctx context.Context, o *orgmodel.Organization) error {
	if err := r.inner.Update(ctx, o); err != nil {
		return err
	}
	r.invalidate(ctx, o.Code)
	return nil
}

func (r *cachedRepository) SoftDelete(ctx context.Context, code string) error {
	if err := r.inner.SoftDelete(ctx, code); err != nil {
		return err
	}
	r.invalidate(ctx, code)
	return nil
}

// CountUsers is not cached: it changes as users are added/removed and is only
// used in the delete-guard (low volume, correctness-sensitive).
func (r *cachedRepository) CountUsers(ctx context.Context, code string) (int64, error) {
	return r.inner.CountUsers(ctx, code)
}

func (r *cachedRepository) set(ctx context.Context, key string, val []byte) {
	if err := r.cache.Set(ctx, key, val, r.ttl); err != nil {
		logger.FromContext(ctx).Warn("org cache: set failed (fail-open)", "key", key, "error", err)
	}
}

func (r *cachedRepository) invalidate(ctx context.Context, code string) {
	if err := r.cache.Delete(ctx, cache.OrgKeys(code)...); err != nil {
		logger.FromContext(ctx).Warn("org cache: invalidate failed (fail-open)", "code", code, "error", err)
	}
}
