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
	// 限制连接数量为1024
	// 达到上限时，不完全阻塞
	// 阻塞到500ms未等到可用连接返回redis错误
	sema, err := redis.NewSemaphore(1024, redis.WithBlock(false), redis.WithWaitDura(500*time.Millisecond))
	if err != nil {
		panic(err)
	}

	// cmd get -> handler
	redis.HandleFunc("GET", func(_ context.Context, w redis.ResponseWriter, r *redis.Request) {
		fmt.Printf("rece get cmd, req %v\n", r)
		w.Text("OK")
	})

	opts := []redis.Option{
		// 连接限制策略
		redis.WithSemaphore(sema),
		// 异常捕获机制
		redis.WithPanicStack(func(format string, args ...interface{}) {
			fmt.Printf("redis rece a panic, err %s", fmt.Sprintf(format, args...))
			debug.PrintStack()
		}),
		// access日志，为监控提供数据源
		redis.WithLoggerFactory(func() redis.Logger { return &logger{} }),
	}

	// 启动Server
	redis.ListenAndServe("0.0.0.0:6379", nil, opts...)
}

func main() {
	ExampleServer()
}
