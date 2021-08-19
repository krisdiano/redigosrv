package main

import (
	"context"
	"fmt"
	"log"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/Saner-Lee/redigosrv/redis"
)

type PanicLoggerAdaptor func(string, ...interface{})

func (fn PanicLoggerAdaptor) Error(msg string, args ...interface{}) {
	fn(msg, args...)
}

type logger struct {
	mu sync.RWMutex
	m  map[string]string
}

func (l *logger) prefix() string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if len(l.m) == 0 {
		return ""
	}

	var secs []string
	for k, v := range l.m {
		secs = append(secs, fmt.Sprintf("%s:%s", k, v))
	}
	return strings.Join(secs, " ")
}

func (l *logger) Debug(s string, args ...interface{}) {
	p := l.prefix()
	log.Printf(p+s, args...)
}

func (l *logger) Info(s string, args ...interface{}) {
	p := l.prefix()
	log.Printf(p+s, args...)
}

func (l *logger) Notice(s string, args ...interface{}) {
	p := l.prefix()
	log.Printf(p+s, args...)
}

func (l *logger) Warn(s string, args ...interface{}) {
	p := l.prefix()
	log.Printf(p+s, args...)
}

func (l *logger) Error(s string, args ...interface{}) {
	p := l.prefix()
	log.Printf(p+s, args...)
}

func (l *logger) AddMeta(k, v string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.m != nil {
		l.m = make(map[string]string)
	}
	l.m[k] = v
}

func ExampleServer() {
	sema, err := redis.NewSemaphore(1024, redis.WithBlock(false), redis.WithWaitDura(500*time.Millisecond))
	if err != nil {
		panic(err)
	}

	redis.HandleFunc(context.Background(), "GET", func(_ context.Context, w redis.ResponseWriter, r *redis.Request) {
		fmt.Printf("rece get cmd, req %v\n", r)
		w.Text("OK")
	})

	// 连接限制策略
	opts := []redis.Option{
		redis.WithSemaphore(sema),
		redis.WithPanicStack(func(format string, args ...interface{}) {
			fmt.Printf("redis rece a panic, err %s", fmt.Sprintf(format, args...))
			debug.PrintStack()
		}),
		redis.WithLoggerFactory(func() redis.Logger { return &logger{} }),
	}

	// 启动Server
	redis.ListenAndServe("0.0.0.0:6379", nil, opts...)
}

func main() {
	ExampleServer()
}
