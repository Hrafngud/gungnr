package service

import (
	"context"
	"sync"
	"time"
)

// A successful read is immutable. Slow refreshes are shared and cannot be canceled
// by one browser leaving; the collector has its own finite deadline.
type cachedHostRead[T any] struct {
	mu         sync.Mutex
	value      T
	valid      bool
	collected  time.Time
	retryAfter time.Time
	pending    chan struct{}
	err        error
}

func (c *cachedHostRead[T]) get(ctx context.Context, ttl time.Duration, load func(context.Context) (T, error)) (T, error) {
	for {
		c.mu.Lock()
		if c.valid && time.Since(c.collected) < ttl {
			v := c.value
			c.mu.Unlock()
			return v, nil
		}
		if c.pending == nil && time.Now().After(c.retryAfter) {
			c.pending = make(chan struct{})
			go c.refresh(load)
		}
		if c.valid {
			v := c.value
			c.mu.Unlock()
			return v, nil
		}
		pending := c.pending
		if pending == nil {
			v, err := c.value, c.err
			c.mu.Unlock()
			return v, err
		}
		c.mu.Unlock()
		select {
		case <-ctx.Done():
			var zero T
			return zero, ctx.Err()
		case <-pending:
		}
	}
}

func (c *cachedHostRead[T]) refresh(load func(context.Context) (T, error)) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	value, err := load(ctx)
	cancel()
	c.mu.Lock()
	defer c.mu.Unlock()
	if err == nil {
		c.value = value
		c.valid = true
		c.collected = time.Now()
	}
	c.err = err
	c.retryAfter = time.Now().Add(time.Second)
	close(c.pending)
	c.pending = nil
}

func (s *HostService) RuntimeSnapshot(ctx context.Context) (HostRuntimeSnapshot, error) {
	return s.snapshotCache.get(ctx, 5*time.Second, s.loadRuntimeSnapshot)
}

func (s *HostService) RuntimeStreamSample(ctx context.Context) (HostRuntimeStreamSample, error) {
	sample, err := s.streamCache.get(ctx, HostRuntimeStreamInterval, s.loadRuntimeStreamSample)
	if err == nil && time.Since(parseCollectionTime(sample.CollectedAt)) > 15*time.Second {
		sample.Warnings = append(append([]string(nil), sample.Warnings...), "Live metrics are delayed; showing the last collected sample.")
	}
	return sample, err
}

func parseCollectionTime(value string) time.Time {
	at, _ := time.Parse(time.RFC3339Nano, value)
	return at
}
