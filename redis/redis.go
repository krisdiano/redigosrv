package redis

import (
	"context"
	"errors"
	"net"
	"runtime/debug"
	"sync"
	"time"
)

type contextKey struct {
	name string
}

var (
	ServerContextKey    = &contextKey{"redis-server"}
	LocalAddrContextKey = &contextKey{"local-addr"}

	ErrServerClosed = errors.New("redis: Server closed")

	shutdownPollInterval = 500 * time.Millisecond
)

type Handler interface {
	ServeRedis(context.Context, ResponseWriter, *Request)
}

type HandlerFunc func(context.Context, ResponseWriter, *Request)

func (f HandlerFunc) ServeRedis(ctx context.Context, w ResponseWriter, r *Request) {
	f(ctx, w, r)
}

type Server struct {
	Addr    string
	Handler Handler

	ConnState func(net.Conn, ConnState)
	opt       option

	inShutdown int32

	mu         sync.Mutex
	onShutdown []func() // Concurrent, protected by mu
	listeners  map[*net.Listener]struct{}
	activeConn map[*conn]struct{}
	doneChan   chan struct{}
}

func ListenAndServe(addr string, handler Handler, opts ...Option) error {
	server := &Server{Addr: addr, Handler: handler}
	return server.ListenAndServe(opts...)
}

func HandleFunc(pattern string, handler func(context.Context, ResponseWriter, *Request)) {
	DefaultServeMux.HandleFunc(pattern, handler)
}

func (srv *Server) ListenAndServe(opts ...Option) error {
	if srv.shuttingDown() {
		return ErrServerClosed
	}

	if len(srv.Addr) == 0 {
		srv.Addr = ":6379"
	}
	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return err
	}
	return srv.Serve(ln, opts...)
}

func (srv *Server) Serve(l net.Listener, opts ...Option) error {
	defer l.Close()

	// Initial
	if srv.Handler == nil {
		srv.Handler = DefaultServeMux
	}
	for _, applyOpt := range opts {
		applyOpt(&srv.opt)
	}

	// Trace lister for release
	if !srv.trackListener(&l, true) {
		return ErrServerClosed
	}
	defer srv.trackListener(&l, false)

	var (
		tempDelay time.Duration
		ctx       = context.WithValue(context.Background(), ServerContextKey, srv)
	)
	for {
		rw, err := l.Accept()
		if err != nil {
			select {
			case <-srv.getDoneChan():
				return ErrServerClosed
			default:
			}

			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				time.Sleep(tempDelay)
				continue
			}
			return err
		}

		tempDelay = 0

		// For close gracefully
		c := srv.newConn(rw)
		c.setState(c.rwc, StateNew)

		// Protocol resolution and routing addressing
		go func() {
			errReply := "-connection limit over\r\n"
			defer func() {
				if err := recover(); err != nil {
					if srv.opt.PanicStack != nil {
						srv.opt.PanicStack(string(debug.Stack()))
					}
					errReply = "-internal failed\r\n"
				}

				c.rwc.Write([]byte(errReply))
				c.setState(c.rwc, StateClosed)
				c.rwc.Close()
			}()

			sema := srv.opt.Sema
			if !sema.opt.Block {
				ok := sema.sema.TryAcquire(1)
				if !ok {
					return
				}
			} else {
				waitCtx, waitCanc := context.WithTimeout(ctx, sema.opt.Wait)
				defer waitCanc()
				err = sema.sema.Acquire(waitCtx, 1)
				if err != nil {
					return
				}
			}
			defer sema.sema.Release(1)
			c.serve(ctx)
		}()
	}
}

func NotFound(ctx context.Context, w ResponseWriter, r *Request) { w.Error("Unsupport cmd") }
func NotFoundHandler() Handler                                   { return HandlerFunc(NotFound) }
