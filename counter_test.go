package goctor

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewCounter(t *testing.T) {
	const min, max = 1, 25000
	const numParallels = 50
	var wg sync.WaitGroup
	wg.Add(numParallels)
	c := NewCounter(WithMinMax(min, max))
	for i := 0; i < numParallels; i++ {
		job := func() {
			for k := 0; k < 1000; k++ {
				c.Add(1)
			}
			wg.Done()
		}
		go job()
	}
	wg.Wait()
	require.Equal(t, int64(1), c.Get())
	c.Set(99)
	require.Equal(t, int64(99), c.Get())
}
