package redis

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type conn struct {
	// server is the server on which the connection arrived.
	// Immutable; never nil.
	server *Server

	// cancelCtx cancels the connection-level context.
	cancelCtx context.CancelFunc

	// rwc is the underlying network connection.
	// This is never wrapped by other types and is the value given out
	// to CloseNotifier callers. It is usually of type *net.TCPConn or
	// *tls.Conn.
	rwc net.Conn

	// remoteAddr is rwc.RemoteAddr().String(). It is not populated synchronously
	// inside the Listener's Accept goroutine, as some implementations block.
	// It is populated immediately inside the (*conn).serve goroutine.
	// This is the value of a Handler's (*Request).RemoteAddr.
	remoteAddr string

	// werr is set to the first write error to rwc.
	// It is set via checkConnErrorWriter{w}, where bufw writes.
	werr error

	// bufr reads from r.
	bufr *bufio.Reader

	curState struct{ atomic uint64 } // packed (unixtime<<8|uint8(ConnState))

	// mu guards hijackedv
	mu sync.Mutex
}

func (srv *Server) newConn(rwc net.Conn) *conn {
	c := &conn{
		server: srv,
		rwc:    rwc,
	}
	return c
}

func (c *conn) setState(nc net.Conn, state ConnState) {
	srv := c.server
	switch state {
	case StateNew:
		srv.trackConn(c, true)
	case StateHijacked, StateClosed:
		srv.trackConn(c, false)
	}
	if state > 0xff || state < 0 {
		panic("internal error")
	}
	packedState := uint64(time.Now().Unix()<<8) | uint64(state)
	atomic.StoreUint64(&c.curState.atomic, packedState)
	if hook := srv.ConnState; hook != nil {
		hook(nc, state)
	}
}

func (c *conn) getState() (state ConnState, unixSec int64) {
	packedState := atomic.LoadUint64(&c.curState.atomic)
	return ConnState(packedState & 0xff), int64(packedState >> 8)
}

var readerPool sync.Pool

func getBufReader(reader io.Reader) *bufio.Reader {
	v := readerPool.Get()
	if v == nil {
		return bufio.NewReaderSize(reader, 1024)
	}
	ret := v.(*bufio.Reader)
	ret.Reset(reader)
	return ret
}

func putBufReader(reader *bufio.Reader) {
	reader.Reset(nil)
	readerPool.Put(reader)
}

func (c *conn) readRequest() (*Request, error) {
	content, _, err := c.bufr.ReadLine()
	if err != nil {
		return nil, err
	}
	if content[0] != '*' {
		return nil, errors.New("bad redis protocol")
	}
	num, err := strconv.Atoi(string(content[1:]))
	if err != nil {
		return nil, errors.New("bad redis protocol")
	}

	var tem []string
	for i := 0; i < num; i++ {
		line, _, err := c.bufr.ReadLine()
		if err != nil {
			return nil, err
		}
		if line[0] != '$' {
			return nil, errors.New("bad redis protocol")
		}

		line1, _, err := c.bufr.ReadLine()
		if err != nil {
			return nil, errors.New("bad redis protocol")
		}
		if fmt.Sprintf("%d", len(line1)) != string(line[1:]) {
			return nil, errors.New("bad redis protocol")
		}
		tem = append(tem, string(line1))
	}
	ret := Request{
		remote: c.remoteAddr,
		Cmd:    strings.ToUpper(tem[0]),
	}
	if len(tem) > 1 {
		ret.Args = tem[1:]
	}
	return &ret, nil
}

func (c *conn) serve(ctx context.Context) {
	c.remoteAddr = c.rwc.RemoteAddr().String()
	ctx = context.WithValue(ctx, LocalAddrContextKey, c.rwc.LocalAddr())
	defer func() {
		c.rwc.Close()
		c.setState(c.rwc, StateClosed)
	}()

	ctx, cancelCtx := context.WithCancel(ctx)
	c.cancelCtx = cancelCtx
	defer cancelCtx()

	c.bufr = getBufReader(c.rwc)
	for {
		req, err := c.readRequest()
		if err != nil {
			return
		}

		req.ts = time.Now()
		if c.server.opt.LoggerFactory != nil {
			req.logger = c.server.opt.LoggerFactory()
		}

		resp := &responser{
			writer: c.rwc,
			req:    *req,
		}

		c.server.Handler.ServeRedis(ctx, resp, req)
	}
}

type ConnState int

const (
	// StateNew represents a new connection that is expected to
	// send a request immediately. Connections begin at this
	// state and then transition to either StateActive or
	// StateClosed.
	StateNew ConnState = iota

	// StateActive represents a connection that has read 1 or more
	// bytes of a request. The Server.ConnState hook for
	// StateActive fires before the request has entered a handler
	// and doesn't fire again until the request has been
	// handled. After the request is handled, the state
	// transitions to StateClosed, StateHijacked, or StateIdle.
	// For HTTP/2, StateActive fires on the transition from zero
	// to one active request, and only transitions away once all
	// active requests are complete. That means that ConnState
	// cannot be used to do per-request work; ConnState only notes
	// the overall state of the connection.
	StateActive

	// StateIdle represents a connection that has finished
	// handling a request and is in the keep-alive state, waiting
	// for a new request. Connections transition from StateIdle
	// to either StateActive or StateClosed.
	StateIdle

	// StateHijacked represents a hijacked connection.
	// This is a terminal state. It does not transition to StateClosed.
	StateHijacked

	// StateClosed represents a closed connection.
	// This is a terminal state. Hijacked connections do not
	// transition to StateClosed.
	StateClosed
)

func (c ConnState) String() string {
	return stateName[c]
}

var stateName = map[ConnState]string{
	StateNew:    "new",
	StateActive: "active",
	StateIdle:   "idle",
	StateClosed: "closed",
}
