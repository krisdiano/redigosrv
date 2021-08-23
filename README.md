# redigosrv

## what

`redis server`侧的`golang`版本实现。

## why

作为后端`RD`，你开发的，维护的最多的应该就是你的`server`，作为服务的提供方来说，想要和客户端进行正常的交互，首先要能理解信息的含义，因此一套通信协议是必须的。

为了选择`redis`协议的，应用层有很多成熟的协议，比如

- http
- http2
- websockets

当然，上述协议都很强大，但是作为内网的服务来说，需要的协议最重要的一点是文本协议，因为文本协议是好理解，便于观察的，而且内网做好权限收敛即可，不需要做加密的二进制协议。

那么为什么不使用`http`呢？`http`很强大，适用范围也比`redis`更广。但是任何工具都是有限制的，比如：

- 服务是存储介质/proxy
- 协议数据包尽可能的小，提高带宽利用率

这时候`redis`协议更加适合！

很多开发者自己实现`redis server`的时候，都是自己解析协议，根据参数找到对应的`handler`，开发者初期通常认为程序很小，没有做抽象，因此最终的实现都耦合在一起，并且一旦规模大起来后很难再维护。

因此你需要一个库帮你屏蔽底层，提高可维护性，它拥有的能力包括：

- 路由注册/寻址
- 连接限制策略
- 支持监控接口
- 中间件能力
- 异常补货
- 优雅关闭

什么，你担心不安全，放心，这是在我在百度项目中已经使用很久的库了，重构的服务支持了**500+**微服务，历经一年没出过性能问题和资源问题。并且解决了当时过于耦合导致的很多性能问题和资源泄露问题。

放心大胆的使用它，如果有什么需求可以直接`issue`搞起，长期维护。


## how

在`go`语言的`net/http`的基础上修改开发，接口完全兼容，将你的学习成本降到最低。在`example`中附带了一个小例子，为了方便阅读，内容直接`copy`到下面了。

```go
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

func main() {
	ExampleServer()
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
```



