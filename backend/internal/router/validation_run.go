package router

import (
	"github.com/gin-gonic/gin"

	"robot-cell-safety-envelope-validator/backend/internal/constants"
	"robot-cell-safety-envelope-validator/backend/internal/handler"
	"robot-cell-safety-envelope-validator/backend/internal/middleware"
)

func registerValidationRunRoutes(group *gin.RouterGroup, target *handler.ValidationRunHandler) {
	routes := group.Group("/validations")
	routes.GET("", target.List)
	routes.GET("/:id", target.Get)
	routes.POST("", middleware.RBAC(constants.RoleSafetyEngineer, constants.RoleAdmin), middleware.RateLimit(30, "simulation"), target.Create)
	review := routes.Group("")
	review.Use(middleware.RBAC(constants.RoleReviewer, constants.RoleAdmin))
	review.POST("/:id/review", target.Review)
	review.POST("/:id/accept", target.Accept)
	review.POST("/:id/void", target.Void)
}
