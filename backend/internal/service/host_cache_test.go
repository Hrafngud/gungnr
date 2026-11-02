package service

import (
	"context"
	"github.com/stretchr/testify/require"
	"sync/atomic"
	"testing"
	"time"
)

func TestSharedHostReadSurvivesOneSubscriberCanceling(t *testing.T) {
	var cache cachedHostRead[string]
	var calls atomic.Int32
	release := make(chan struct{})
	load := func(ctx context.Context) (string, error) {
		calls.Add(1)
		select {
		case <-release:
			return "sample", nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	firstCtx, cancel := context.WithCancel(context.Background())
	first := make(chan error, 1)
	go func() { _, err := cache.get(firstCtx, time.Minute, load); first <- err }()
	require.Eventually(t, func() bool { return calls.Load() == 1 }, time.Second, time.Millisecond)
	second := make(chan string, 1)
	go func() {
		v, err := cache.get(context.Background(), time.Minute, load)
		if err != nil {
			second <- err.Error()
			return
		}
		second <- v
	}()
	cancel()
	select {
	case err := <-first:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("subscriber did not cancel")
	}
	close(release)
	select {
	case v := <-second:
		require.Equal(t, "sample", v)
	case <-time.After(time.Second):
		t.Fatal("shared collection was canceled")
	}
	require.Equal(t, int32(1), calls.Load())
}
