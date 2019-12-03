package vpcinfo

import (
	"context"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRegistryLookupPlatform(t *testing.T) {
	p, err := LookupPlatform()
	if err != nil {
		if os.IsPermission(err) {
			t.Skip(err)
		}
		t.Fatal(err)
	}
	switch p.(type) {
	case aws, unknown:
		t.Log("platform:", p)
	default:
		t.Error("unrecognized platform:", p)
	}
}

func TestRegistryLookupZone(t *testing.T) {
	z, err := LookupZone()
	if err != nil {
		if os.IsPermission(err) {
			t.Skip(err)
		}
		t.Fatal(err)
	}
	t.Log("zone:", z)
}

func TestRegistryLookupSubnets(t *testing.T) {
	cacheMisses := uint32(0)

	r := &Registry{
		Resolver: resolverFunc(func(ctx context.Context, _ string) ([]string, error) {
			atomic.AddUint32(&cacheMisses, 1)
			delay := time.NewTimer(10 * time.Millisecond)
			defer delay.Stop()
			select {
			case <-delay.C:
				return testTXT[:], nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}),
		Timeout: 1 * time.Second,
		TTL:     100 * time.Millisecond,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	const N = 100
	wg := sync.WaitGroup{}

	for a := 0; a < 2; a++ {
		for i := 0; i < N; i++ {
			wg.Add(1)
			go func(ctx context.Context) {
				defer wg.Done()
				if _, err := r.LookupSubnets(ctx); err != nil {
					t.Error(err)
				}
			}(ctx)
		}

		wg.Wait()

		if a == 0 {
			time.Sleep(120 * time.Millisecond) // wait so the cache expires
		}
	}

	if miss := atomic.LoadUint32(&cacheMisses); miss != 4 {
		t.Error("invalid cache misses")
		t.Log("expected: 4")
		t.Log("found:   ", miss)
	}
}

func BenchmarkRegistry(b *testing.B) {
	r := &Registry{
		Resolver: resolverFunc(func(ctx context.Context, _ string) ([]string, error) {
			return testTXT[:], nil
		}),
	}

	ctx := context.Background()

	for i := 0; i < b.N; i++ {
		_, _ = r.LookupSubnets(ctx)
	}
}
