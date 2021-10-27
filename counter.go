package goctor

import (
	"math"
	"sync"
	"sync/atomic"
)

type options struct {
	min, max int64
}

type Option func(*options)

func WithMinMax(min, max int64) Option {
	return func(opts *options) {
		opts.min = min
		opts.max = max
	}
}

type Counter interface {
	Add(delta int64) int64
	Get() int64
	Set(num int64)
}

type counter struct {
	mux      sync.RWMutex
	num      int64
	min, max int64
}

func NewCounter(opts ...Option) Counter {
	dopt := &options{
		min: 0,
		max: math.MaxInt64,
	}
	for i := range opts {
		opts[i](dopt)
	}
	return &counter{
		num: dopt.min,
		min: dopt.min,
		max: dopt.max,
	}
}

func (c *counter) Set(v int64) {
	c.mux.Lock()
	defer c.mux.Unlock()
	atomic.StoreInt64(&c.num, v)
}

func (c *counter) Get() int64 {
	c.mux.RLock()
	defer c.mux.RUnlock()
	return atomic.LoadInt64(&c.num)
}

func (c *counter) Add(delta int64) int64 {
	c.mux.Lock()
	defer c.mux.Unlock()
	num := atomic.AddInt64(&c.num, delta)
	if num > c.max {
		atomic.StoreInt64(&c.num, c.min)
		return c.min
	}
	return num
}
