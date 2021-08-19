package redis

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"time"
)

type ResponseWriter interface {
	Error(string, ...interface{}) (int, error)
	Text(string, ...interface{}) (int, error)
	Binary([]byte) (int, error)

	Write([]byte) (int, error)
}

type responser struct {
	writer io.Writer
	req    Request
}

func (resp *responser) Error(msg string, args ...interface{}) (int, error) {
	content := fmt.Sprintf(msg, args...)
	cmd := fmt.Sprintf("-%s\r\n", content)
	defer func() {
		if logger := resp.req.logger; logger != nil {
			logger.Notice("redis cmd:%s, arg:%v, addr:%s, cost:%v, type:err, reply:%s", resp.req.Cmd, resp.req.Args, resp.req.remote, time.Now().Sub(resp.req.ts), content)
		}
	}()
	return resp.writer.Write([]byte(cmd))
}

func (resp *responser) Text(msg string, args ...interface{}) (int, error) {
	content := fmt.Sprintf(msg, args...)
	cmd := fmt.Sprintf("+%s\r\n", content)
	defer func() {
		if logger := resp.req.logger; logger != nil {
			logger.Notice("redis cmd:%s, arg:%v, addr:%s, cost:%v, type:text, reply:%s", resp.req.Cmd, resp.req.Args, resp.req.remote, time.Now().Sub(resp.req.ts), content)
		}
	}()
	return resp.writer.Write([]byte(cmd))
}

func (resp *responser) Binary(msg []byte) (int, error) {
	if len(msg) != 0 {
		var sb bytes.Buffer
		sb.WriteByte('$')
		sb.WriteString(strconv.FormatInt(int64(len(msg)), 10))
		sb.WriteString("\r\n")
		sb.Write(msg)
		sb.WriteString("\r\n")

		defer func() {
			if logger := resp.req.logger; logger != nil {
				logger.Notice("redis cmd:%s, arg:%v, addr:%s, cost:%v, type:bulkstr, reply:%s", resp.req.Cmd, resp.req.Args, resp.req.remote, time.Now().Sub(resp.req.ts), string(msg))
			}
		}()
		return resp.writer.Write(sb.Bytes())
	}

	defer func() {
		if logger := resp.req.logger; logger != nil {
			logger.Notice("redis cmd:%s, arg:%v, addr:%s, cost:%v, type:null bulkstr, reply:%s", resp.req.Cmd, resp.req.Args, resp.req.remote, time.Now().Sub(resp.req.ts), "numm bulk string")
		}
	}()
	return resp.writer.Write([]byte{'$', '-', '1', '\r', '\n'})

}

func (resp *responser) Write(bins []byte) (int, error) {
	if len(bins) == 0 {
		return 0, nil
	}

	var (
		target = len(bins)
		writen int
	)
	for writen < target {
		cnt, err := resp.writer.Write(bins)
		writen += cnt
		if err == nil || err == io.ErrShortWrite {
			continue
		}
		return writen, err
	}
	return target, nil
}
