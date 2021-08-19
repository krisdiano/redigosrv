package redis

type Logger interface {
	Debug(string, ...interface{})
	Info(string, ...interface{})
	Notice(string, ...interface{})
	Warn(string, ...interface{})
	Error(string, ...interface{})

	AddMeta(string, string)
}
