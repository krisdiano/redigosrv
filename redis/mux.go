package redis

import (
	"context"
	"strings"
	"sync"
)

var (
	DefaultServeMux = &defaultServeMux
	defaultServeMux ServeMux
)

type ServeMux struct {
	mu     sync.RWMutex
	router map[string]muxEntry
}

type muxEntry struct {
	pattern string
	h       Handler
}

func (mux *ServeMux) ServeRedis(ctx context.Context, w ResponseWriter, r *Request) {
	mux.mu.RLock()
	defer mux.mu.RUnlock()

	entry, ok := mux.router[r.Cmd]
	if !ok {
		NotFoundHandler().ServeRedis(ctx, w, r)
		return
	}

	h := entry.h
	h.ServeRedis(ctx, w, r)
}

func (mux *ServeMux) HandleFunc(ctx context.Context, pattern string, handler func(context.Context, ResponseWriter, *Request), middlewares ...MiddlewareFunc) {
	if handler == nil {
		panic("redis: nil handler")
	}
	if len(middlewares) > 0 {
		handler = MergeMiddleware(handler, middlewares...)
	}
	mux.Handle(pattern, HandlerFunc(handler))
}

func (mux *ServeMux) Handle(pattern string, handler Handler) {
	mux.mu.Lock()
	defer mux.mu.Unlock()

	if pattern == "" {
		panic("redis: invalid pattern")
	}
	if handler == nil {
		panic("redis: nil handler")
	}
	if _, exist := mux.router[pattern]; exist {
		panic("redis: multiple registrations for " + pattern)
	}

	if mux.router == nil {
		mux.router = make(map[string]muxEntry)
	}
	pattern = strings.ToUpper(pattern)
	e := muxEntry{h: handler, pattern: pattern}
	mux.router[pattern] = e
}
