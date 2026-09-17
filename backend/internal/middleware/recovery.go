package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
)

func Recovery() gin.HandlerFunc {
	return func(context *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				slog.Error("panic recovered", "request_id", context.GetString("request_id"), "panic", recovered, "stack", string(debug.Stack()))
				context.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error":      gin.H{"code": "internal_error", "message": "unexpected server error"},
					"request_id": context.GetString("request_id"),
				})
			}
		}()
		context.Next()
	}
}
