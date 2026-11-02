package worker

import (
	"context"
	"errors"
	"sync"
	"time"
)

var errDiskCollectionPending = errors.New("disk accounting is being collected in the background")

const containerDiskCacheKey = "container-sizes"
const systemDiskCacheKey = "system-df"

type diskCommandEntry struct {
	output      []byte
	collectedAt time.Time
	attemptedAt time.Time
	collecting  bool
	err         error
}

type diskCommandCache struct {
	mu      sync.Mutex
	entries map[string]*diskCommandEntry
	lane    chan struct{}
	ctx     context.Context
}

func newDiskCommandCache(ctx context.Context) *diskCommandCache {
	return &diskCommandCache{entries: make(map[string]*diskCommandEntry), lane: make(chan struct{}, 1), ctx: ctx}
}

// Return the last successful scan immediately and refresh it at most once a minute.
// A cold cache is explicitly pending, never a fabricated zero-byte measurement.
func (c *diskCommandCache) get(key string, load func(context.Context) ([]byte, error)) ([]byte, time.Time, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry := c.entries[key]
	if entry == nil {
		entry = &diskCommandEntry{}
		c.entries[key] = entry
	}
	if !entry.collecting && time.Since(entry.collectedAt) >= time.Minute && time.Since(entry.attemptedAt) >= 10*time.Second {
		entry.collecting = true
		entry.attemptedAt = time.Now()
		go c.refresh(entry, load)
	}
	if !entry.collectedAt.IsZero() {
		return append([]byte(nil), entry.output...), entry.collectedAt, nil
	}
	if entry.err != nil {
		return nil, time.Time{}, entry.err
	}
	return nil, time.Time{}, errDiskCollectionPending
}

func (c *diskCommandCache) refresh(entry *diskCommandEntry, load func(context.Context) ([]byte, error)) {
	ctx, cancel := context.WithTimeout(c.ctx, 90*time.Second)
	defer cancel()
	var output []byte
	var err error
	select {
	case c.lane <- struct{}{}:
		output, err = load(ctx)
		<-c.lane
	case <-ctx.Done():
		err = ctx.Err()
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	entry.collecting = false
	entry.err = err
	if err == nil {
		entry.output = append([]byte(nil), output...)
		entry.collectedAt = time.Now().UTC()
	}
}

func (c *diskCommandCache) state(key string) (time.Time, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry := c.entries[key]
	if entry == nil {
		return time.Time{}, nil
	}
	return entry.collectedAt, entry.err
}

type cachedDiskExecutor struct {
	base  commandExecutor
	cache *diskCommandCache
}

func (e cachedDiskExecutor) Run(ctx context.Context, req commandRequest) ([]byte, error) {
	if req.Name == "docker" && len(req.Args) > 1 && req.Args[0] == "ps" && req.Args[1] == "-as" {
		output, _, err := e.cache.get(containerDiskCacheKey, func(background context.Context) ([]byte, error) { return e.base.Run(background, req) })
		return output, err
	}
	return e.base.Run(ctx, req)
}
