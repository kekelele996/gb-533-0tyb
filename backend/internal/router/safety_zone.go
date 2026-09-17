package router

import (
	"github.com/gin-gonic/gin"

	"robot-cell-safety-envelope-validator/backend/internal/constants"
	"robot-cell-safety-envelope-validator/backend/internal/handler"
	"robot-cell-safety-envelope-validator/backend/internal/middleware"
)

func registerSafetyZoneRoutes(group *gin.RouterGroup, target *handler.SafetyZoneHandler) {
	routes := group.Group("/zones")
	routes.GET("", target.List)
	routes.GET("/:id", target.Get)
	write := routes.Group("")
	write.Use(middleware.RBAC(constants.RoleSafetyEngineer, constants.RoleAdmin))
	write.POST("", target.Create)
	write.PUT("/:id", target.Update)
	write.POST("/:id/activate", target.Activate)
	write.POST("/:id/deactivate", target.Deactivate)
}
