package router

import (
	"github.com/gin-gonic/gin"

	"robot-cell-safety-envelope-validator/backend/internal/constants"
	"robot-cell-safety-envelope-validator/backend/internal/handler"
	"robot-cell-safety-envelope-validator/backend/internal/middleware"
)

func registerMotionProgramRoutes(group *gin.RouterGroup, target *handler.MotionProgramHandler) {
	routes := group.Group("/programs")
	routes.GET("", target.List)
	routes.GET("/:id", target.Get)
	write := routes.Group("")
	write.Use(middleware.RBAC(constants.RoleRobotProgrammer, constants.RoleAdmin))
	write.POST("", middleware.RateLimit(20, "program_import"), target.Create)
	write.POST("/:id/transition", target.Transition)
}
