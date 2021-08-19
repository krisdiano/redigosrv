package redis

import "time"

type Request struct {
	Cmd  string
	Args []string

	logger Logger
	remote string
	ts     time.Time
}

func (r *Request) Logger() Logger {
	return r.logger
}

func (r *Request) Remote() string {
	return r.remote
}
