package router

import (
	"github.com/gin-gonic/gin"

	"robot-cell-safety-envelope-validator/backend/internal/constants"
	"robot-cell-safety-envelope-validator/backend/internal/handler"
	"robot-cell-safety-envelope-validator/backend/internal/middleware"
)

func registerRobotCellRoutes(group *gin.RouterGroup, target *handler.RobotCellHandler) {
	routes := group.Group("/cells")
	routes.GET("", target.List)
	routes.GET("/:id", target.Get)
	write := routes.Group("")
	write.Use(middleware.RBAC(constants.RoleSafetyEngineer, constants.RoleAdmin))
	write.POST("", target.Create)
	write.PUT("/:id", target.Update)
	write.POST("/:id/freeze", target.Freeze)
	write.POST("/:id/deactivate", target.Deactivate)
}
