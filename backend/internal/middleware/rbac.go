package middleware

import (
	"github.com/gin-gonic/gin"

	"robot-cell-safety-envelope-validator/backend/internal/dto"
	"robot-cell-safety-envelope-validator/backend/internal/service"
)

func RBAC(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]bool, len(roles))
	for _, role := range roles {
		allowed[role] = true
	}
	return func(context *gin.Context) {
		value, exists := context.Get(actorContextKey)
		actor, valid := value.(dto.Actor)
		if !exists || !valid {
			writeAuthError(context, service.Unauthorized("authentication context is missing"))
			return
		}
		if !allowed[actor.Role] {
			writeAuthError(context, service.Forbidden("role is not allowed to perform this action"))
			return
		}
		context.Next()
	}
}
