package goctor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func newEmptyActor() Actor {
	return &CountActor{id: "id"}
}

type CountActor struct {
	id      string
	value   int
	started bool
	stopped bool
	timer   bool
}

func (e *CountActor) Receive(m Message) {
	switch m.(type) {
	case *Started:
		e.started = true
	case *Stopped:
		e.stopped = true
	case *Timer:
		e.timer = true
	default:
		e.value++
	}
}

func newEmptyMessage(id string) *emptyMessage {
	return &emptyMessage{id: id}
}

func TestActor(t *testing.T) {
	m := &emptyMessage{}

	// new
	p := NewProcessFromActor(newEmptyActor)
	require.Equal(t, 1024, cap(p.mailbox))
	require.False(t, p.sharding)
	require.NoError(t, p.Send(m))
	p.Stop(context.Background())

	// new with capacity
	p = NewProcessFromActor(newEmptyActor, WithMailboxCapacity(10))
	require.Equal(t, 10, cap(p.mailbox))
	require.NoError(t, p.Send(m))
	p.Stop(context.Background())

	// new with capacity
	p = NewProcessFromActor(newEmptyActor, WithTimer(time.Second))
	require.Equal(t, time.Second, p.opts.timer)
	require.NoError(t, p.Send(m))
	p.Stop(context.Background())

	// new with range based sharding
	p = NewProcessFromActor(newEmptyActor,
		WithRangeBasedSharding(10, 1),
	)
	require.NoError(t, p.Send(newEmptyMessage("ab")))
	require.NoError(t, p.Send(newEmptyMessage("ab")))
	require.NoError(t, p.Send(newEmptyMessage("ac")))
	require.NoError(t, p.Send(newEmptyMessage("ba")))
	require.NoError(t, p.Send(newEmptyMessage("cb")))
	require.NoError(t, p.Send(newEmptyMessage("db")))
	p.Stop(context.Background())

	// new with round robin sharding
	p = NewProcessFromActor(newEmptyActor,
		WithRoundRobinSharding(5),
	)
	require.Len(t, p.shards, 5)
	require.NoError(t, p.Send(newEmptyMessage("ab")))
	require.NoError(t, p.Send(newEmptyMessage("ab")))
	require.NoError(t, p.Send(newEmptyMessage("ac")))
	require.NoError(t, p.Send(newEmptyMessage("ba")))
	require.NoError(t, p.Send(newEmptyMessage("cb")))
	require.NoError(t, p.Send(newEmptyMessage("db")))
	p.Stop(context.Background())

	// unknown sharding type
	p = NewProcessFromActor(newEmptyActor,
		WithRangeBasedSharding(10, 1),
	)
	p.opts.rangeBasedSharding = false
	require.Error(t, p.Send(newEmptyMessage("ab")))
	p.Stop(context.Background())

	// life cycle
	actor := &CountActor{}
	p = NewProcessFromActor(func() Actor { return actor },
		WithTimer(time.Second),
	)
	require.NoError(t, p.Send(newEmptyMessage("id")))
	require.NoError(t, p.Send(newEmptyMessage("id")))
	time.Sleep(2 * time.Second)
	p.Stop(context.Background())
	require.Equal(t, 2, actor.value)
	require.True(t, actor.started)
	require.True(t, actor.stopped)
	require.True(t, actor.timer)
}
