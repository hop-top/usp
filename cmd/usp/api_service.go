package main

import (
	"fmt"
	"time"

	"hop.top/usp/internal/api"
)

func newAPIService() (*api.Service, error) {
	ttlText := rootViper.GetString("cache_ttl")
	if ttlText == "" {
		ttlText = defaultConfig().CacheTTL
	}
	ttl, err := time.ParseDuration(ttlText)
	if err != nil {
		return nil, fmt.Errorf("invalid cache_ttl %q: %w", ttlText, err)
	}
	cache, err := api.OpenDefaultCache(ttl)
	if err != nil {
		return api.NewDefault(), nil
	}
	return api.NewDefault(api.WithCache(cache)), nil
}
