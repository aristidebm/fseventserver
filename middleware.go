package fseventserver

import (
	"context"
	"fmt"
	"log/slog"
)

func LoggingMiddleware(h Handler) Handler {
	return HandlerFunc(func(ctx context.Context) error {
		if req, ok := ctx.Value("request").(*Request); ok {
			slog.Info(fmt.Sprintf("%+v", req))
		}
		return h.ServeFSEvent(ctx)
	})
}

func Use(han Handler, mid ...Middleware) Handler {
	temp := han
	for _, m := range mid {
		temp = m(temp)
	}
	return temp
}
