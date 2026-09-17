package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

func Audit() gin.HandlerFunc {
	return func(context *gin.Context) {
		started := time.Now()
		context.Next()
		route := context.FullPath()
		if route == "" {
			route = "unmatched"
		}
		slog.Info("http_request",
			"request_id", context.GetString("request_id"), "method", context.Request.Method,
			"route", route, "status", context.Writer.Status(), "latency_ms", time.Since(started).Milliseconds(),
			"client_ip", context.ClientIP(), "response_bytes", context.Writer.Size(),
		)
	}
}
