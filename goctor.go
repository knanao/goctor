package goctor

import (
	"context"
	"errors"
	"time"
)

const (
	messageTypeStarted = "started"
	messageTypeStopped = "stopped"
	messageTypeTimer   = "timer"
	messageTypeMessage = "message"
)

var (
	ErrBounded             = errors.New("goctor: message is bounded")
	ErrEmptyID             = errors.New("goctor: ID is empty")
	ErrUnsupportedID       = errors.New("goctor: ID is unsupported")
	ErrUnknownShardingType = errors.New("goctor: send to unknown sharding type")
)

type Process struct {
	ctx          context.Context
	opts         *processOptions
	mailbox      chan Message
	sharding     bool
	shards       []*Process
	shardCounter Counter
	timer        *time.Ticker
}

func (p *Process) Stop(ctx context.Context) {
	if !p.sharding {
		p.stop(ctx)
		return
	}
	p.stopShards(ctx)
}

func (p *Process) stop(ctx context.Context) {
	p.stopTimer()
	close(p.mailbox)
	select {
	case <-ctx.Done():
	case <-p.ctx.Done():
	}
}

func (p *Process) stopShards(ctx context.Context) {
	for i := range p.shards {
		p.shards[i].stopTimer()
		close(p.shards[i].mailbox)
	}
	for i := range p.shards {
		select {
		case <-ctx.Done():
			return
		case <-p.shards[i].ctx.Done():
		}
	}
}

func (p *Process) stopTimer() {
	if p.opts.timer == 0 {
		return
	}
	p.timer.Stop()
}

func (p *Process) Send(msg Message) error {
	if !p.sharding {
		return p.send(msg)
	}
	switch {
	case p.opts.rangeBasedSharding:
		return p.sendWithRangeBasedSharding(msg)
	case p.opts.roundRobinSharding:
		return p.sendWithRoundRobinSharding(msg)
	}
	return ErrUnknownShardingType
}

func (p *Process) send(msg Message) error {
	select {
	case p.mailbox <- msg:
		return nil
	default:
		return ErrBounded
	}
}

func (p *Process) sendWithRangeBasedSharding(msg Message) error {
	const start, end = '0', 'z'
	id := []rune(msg.ID())
	if len(id) == 0 {
		return ErrEmptyID
	}
	if id[0] < start || id[0] > end {
		return ErrUnsupportedID
	}
	num := int(id[0] - '0')
	num /= p.opts.chunkSize
	num %= p.opts.numShards
	return p.shards[num].send(msg)
}

func (p *Process) sendWithRoundRobinSharding(msg Message) error {
	num := p.shardCounter.Add(1)
	return p.shards[num].send(msg)
}

type Actor interface {
	Receive(msg Message)
}

type processOptions struct {
	name               string
	mailboxCap         int
	rangeBasedSharding bool
	roundRobinSharding bool
	numShards          int
	chunkSize          int
	timer              time.Duration
}

type ProcessOption func(*processOptions)

func WithMailboxCapacity(v int) ProcessOption {
	return func(o *processOptions) {
		o.mailboxCap = v
	}
}

// WithRangeBasedSharding shards within the range of the first character of the message ID.
func WithRangeBasedSharding(numShards, chunkSize int) ProcessOption {
	return func(o *processOptions) {
		o.rangeBasedSharding = true
		o.numShards = numShards
		o.chunkSize = chunkSize
	}
}

// WithRoundRobinSharding Sharding in order from 0th.
func WithRoundRobinSharding(numShards int) ProcessOption {
	return func(o *processOptions) {
		o.roundRobinSharding = true
		o.numShards = numShards
	}
}

func WithTimer(d time.Duration) ProcessOption {
	return func(o *processOptions) {
		o.timer = d
	}
}

func NewProcessFromActor(a func() Actor, opts ...ProcessOption) *Process {
	const defaultMailboxCap = 1024
	dopt := &processOptions{
		mailboxCap: defaultMailboxCap,
	}
	ctx := context.Background()
	for i := range opts {
		opts[i](dopt)
	}
	if dopt.numShards == 0 {
		return newProcessFromActor(ctx, a(), dopt)
	}
	return newShardingProcess(ctx, a, dopt)
}

func newShardingProcess(ctx context.Context, a func() Actor, opts *processOptions) *Process {
	processes := make([]*Process, opts.numShards)
	for i := range processes {
		processes[i] = newProcessFromActor(ctx, a(), opts)
	}
	opt := WithMinMax(0, int64(opts.numShards)-1)
	root := &Process{
		opts:         opts,
		sharding:     true,
		shards:       processes,
		shardCounter: NewCounter(opt),
	}
	return root
}

func newProcessFromActor(ctx context.Context, a Actor, opts *processOptions) *Process {
	cancelCtx, cancel := context.WithCancel(ctx)
	p := &Process{
		ctx:     cancelCtx,
		opts:    opts,
		mailbox: make(chan Message, opts.mailboxCap),
	}
	p.timer = &time.Ticker{C: make(chan time.Time, 1)}
	if opts.timer > 0 {
		p.timer = time.NewTicker(opts.timer)
	}
	receive := func() {
		a.Receive(startedMessage)
		defer cancel()
		defer a.Receive(stoppedMessage)
		for {
			select {
			case msg, ok := <-p.mailbox:
				if !ok {
					return
				}
				a.Receive(msg)
			case <-p.timer.C:
				a.Receive(timerMessage)
			}
		}
	}
	go receive()
	return p
}
