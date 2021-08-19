package redis

import (
	"errors"
	"time"

	sem "golang.org/x/sync/semaphore"
)

var (
	defaultBlock bool          = false
	defaultWait  time.Duration = time.Millisecond * 100
)

func defaultSemaphore() *Semaphore {
	return &Semaphore{
		sema: sem.NewWeighted(1000),
		opt: semaOpt{
			Block: defaultBlock,
			Wait:  defaultWait,
		},
	}
}

type option struct {
	PanicStack    func(string, ...interface{})
	LoggerFactory func() Logger
	Sema          *Semaphore
}

type Option func(*option)

func WithPanicStack(fn func(string, ...interface{})) Option {
	return func(o *option) {
		o.PanicStack = fn
	}
}

func WithLoggerFactory(genFn func() Logger) Option {
	return func(o *option) {
		o.LoggerFactory = genFn
	}
}

func WithSemaphore(s *Semaphore) Option {
	return func(o *option) {
		o.Sema = s
	}
}

type Semaphore struct {
	sema *sem.Weighted
	opt  semaOpt
}

type semaOpt struct {
	Block bool
	Wait  time.Duration
}

type SemaOpt func(*semaOpt)

func WithBlock(block bool) SemaOpt {
	return func(o *semaOpt) {
		o.Block = block
	}
}

// if block is false, this config should not be used
func WithWaitDura(d time.Duration) SemaOpt {
	var wait time.Duration
	if d <= 0 {
		wait = defaultWait
	} else {
		wait = d
	}
	return func(o *semaOpt) {
		o.Wait = wait
	}
}

func NewSemaphore(n int64, opts ...SemaOpt) (*Semaphore, error) {
	if n < 1 {
		return nil, errors.New("n for semaphore should ge 1")
	}

	ret := &Semaphore{sema: sem.NewWeighted(n)}
	for _, optFn := range opts {
		optFn(&ret.opt)
	}
	return ret, nil
}
