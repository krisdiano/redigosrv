package redis

type MiddlewareFunc func(HandlerFunc) HandlerFunc

func MergeMiddleware(fn HandlerFunc, middlewares ...MiddlewareFunc) HandlerFunc {
	if len(middlewares) == 0 {
		return fn
	}
	for _, m := range middlewares {
		fn = m(fn)
	}
	return fn
}
