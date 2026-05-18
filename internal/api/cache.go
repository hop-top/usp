package api

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"hop.top/kit/go/core/xdg"
	"hop.top/kit/go/storage/sqlstore"
)

// Cache is the API's narrow cache boundary. The default implementation
// is kit/storage/sqlstore, but tests can provide an in-memory equivalent.
type Cache interface {
	Get(ctx context.Context, key string, dst any) (bool, error)
	Put(ctx context.Context, key string, v any) error
}

type closeCache interface {
	Close() error
}

// OpenCache opens a kit-backed SQLite cache at path.
func OpenCache(path string, ttl time.Duration) (*sqlstore.Store, error) {
	return sqlstore.Open(path, sqlstore.Options{TTL: ttl})
}

// OpenDefaultCache opens usp's XDG cache database.
func OpenDefaultCache(ttl time.Duration) (*sqlstore.Store, error) {
	path, err := xdg.CacheFile("usp", "api-cache.db")
	if err != nil {
		return nil, err
	}
	return OpenCache(path, ttl)
}

func cacheKey(op string, req any) string {
	payload, err := json.Marshal(req)
	if err != nil {
		return "api:" + op
	}
	sum := sha256.Sum256(payload)
	return fmt.Sprintf("api:%s:%x", op, sum)
}

func (s *Service) cacheGet(ctx context.Context, op string, req any, dst any) bool {
	if s.cache == nil {
		return false
	}
	ok, err := s.cache.Get(ctx, cacheKey(op, req), dst)
	return err == nil && ok
}

func (s *Service) cachePut(ctx context.Context, op string, req any, value any) {
	if s.cache == nil || ctx.Err() != nil {
		return
	}
	_ = s.cache.Put(ctx, cacheKey(op, req), value)
}
