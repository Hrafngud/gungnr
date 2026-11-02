package worker

import (
	"context"
	"errors"
	"github.com/stretchr/testify/require"
	"sync/atomic"
	"testing"
	"time"
)

func TestDiskCacheReturnsPendingAndSharesOneScan(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cache := newDiskCommandCache(ctx)
	var calls atomic.Int32
	release := make(chan struct{})
	load := func(ctx context.Context) ([]byte, error) {
		calls.Add(1)
		select {
		case <-release:
			return []byte("sizes"), nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	for i := 0; i < 30; i++ {
		_, _, err := cache.get(containerDiskCacheKey, load)
		require.ErrorIs(t, err, errDiskCollectionPending)
	}
	require.Eventually(t, func() bool { return calls.Load() == 1 }, time.Second, time.Millisecond)
	close(release)
	require.Eventually(t, func() bool { at, _ := cache.state(containerDiskCacheKey); return !at.IsZero() }, time.Second, time.Millisecond)
	output, at, err := cache.get(containerDiskCacheKey, load)
	require.NoError(t, err)
	require.Equal(t, "sizes", string(output))
	require.False(t, at.IsZero())
	require.Equal(t, int32(1), calls.Load())
}

func TestDiskCacheKeepsLastSuccessWhenRefreshFails(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cache := newDiskCommandCache(ctx)
	old := time.Now().Add(-2 * time.Minute)
	cache.entries[containerDiskCacheKey] = &diskCommandEntry{output: []byte("old sizes"), collectedAt: old}
	release := make(chan struct{})
	load := func(ctx context.Context) ([]byte, error) {
		select {
		case <-release:
			return nil, errors.New("daemon unavailable")
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	output, at, err := cache.get(containerDiskCacheKey, load)
	require.NoError(t, err)
	require.Equal(t, "old sizes", string(output))
	require.Equal(t, old, at)
	close(release)
	require.Eventually(t, func() bool { _, err := cache.state(containerDiskCacheKey); return err != nil }, time.Second, time.Millisecond)
	output, at, err = cache.get(containerDiskCacheKey, load)
	require.NoError(t, err)
	require.Equal(t, "old sizes", string(output))
	require.Equal(t, old, at)
}
