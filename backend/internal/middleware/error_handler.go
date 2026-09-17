package middleware

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

func ErrorHandler() gin.HandlerFunc {
	return func(context *gin.Context) {
		context.Header("X-Content-Type-Options", "nosniff")
		context.Header("X-Frame-Options", "DENY")
		context.Header("Referrer-Policy", "no-referrer")
		context.Next()
		if len(context.Errors) == 0 || context.Writer.Written() {
			return
		}
		last := context.Errors.Last()
		slog.Error("unhandled request error", "request_id", context.GetString("request_id"), "error", last.Error())
		context.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"error":      gin.H{"code": "internal_error", "message": "unexpected server error"},
			"request_id": context.GetString("request_id"),
		})
	}
}
