package lyrics

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/f1nniboy/chorus/internal/cache"
)

type Fetcher struct {
	provider Provider
	cache    *cache.Cache
}

func NewFetcher(p Provider, c *cache.Cache) *Fetcher {
	return &Fetcher{provider: p, cache: c}
}

func (f *Fetcher) Get(ctx context.Context, q TrackQuery) (Result, error) {
	key := cache.Key(f.provider.ID(), q.Artist, q.Title, q.Album)

	if data, ok := f.cache.Get(key); ok {
		var res Result
		if err := json.Unmarshal(data, &res); err == nil {
			return res, nil
		}
	}

	return f.fetchAndCache(ctx, q, key)
}

func (f *Fetcher) Refetch(ctx context.Context, q TrackQuery) (Result, error) {
	key := cache.Key(f.provider.ID(), q.Artist, q.Title, q.Album)
	return f.fetchAndCache(ctx, q, key)
}

func (f *Fetcher) Save(q TrackQuery, res Result) error {
	data, err := json.Marshal(res)
	if err != nil {
		return err
	}
	key := cache.Key(f.provider.ID(), q.Artist, q.Title, q.Album)
	return f.cache.Set(key, data)
}

func (f *Fetcher) fetchAndCache(ctx context.Context, q TrackQuery, key string) (Result, error) {
	res, err := f.provider.Fetch(ctx, q)
	if err != nil {
		return Result{}, err
	}

	data, err := json.Marshal(res)
	if err != nil {
		return Result{}, err
	}
	if err := f.cache.Set(key, data); err != nil {
		slog.Warn("lyrics: failed to cache fetched lyrics", "err", err)
	}
	return res, nil
}
