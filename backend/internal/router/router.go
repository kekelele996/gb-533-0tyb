package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"robot-cell-safety-envelope-validator/backend/internal/config"
	"robot-cell-safety-envelope-validator/backend/internal/constants"
	"robot-cell-safety-envelope-validator/backend/internal/handler"
	"robot-cell-safety-envelope-validator/backend/internal/middleware"
	"robot-cell-safety-envelope-validator/backend/internal/repository"
	"robot-cell-safety-envelope-validator/backend/internal/service"
)

type handlers struct {
	system      *handler.SystemHandler
	cells       *handler.RobotCellHandler
	zones       *handler.SafetyZoneHandler
	programs    *handler.MotionProgramHandler
	validations *handler.ValidationRunHandler
	auth        *service.SystemService
}

func New(db *gorm.DB, cfg config.Config) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(middleware.RequestID(), middleware.ErrorHandler(), middleware.Recovery(), middleware.CORS(cfg.CORSOrigin), middleware.RateLimit(cfg.RateLimitPerMinute, "global"), middleware.Audit())
	wired := wire(db, cfg)
	engine.GET("/healthz", wired.system.Health)
	engine.GET("/readyz", wired.system.Ready)
	api := engine.Group("/api/v1")
	api.POST("/auth/login", middleware.RateLimit(30, "login"), wired.system.Login)
	protected := api.Group("")
	protected.Use(middleware.Auth(wired.auth))
	registerRobotCellRoutes(protected, wired.cells)
	registerSafetyZoneRoutes(protected, wired.zones)
	registerMotionProgramRoutes(protected, wired.programs)
	registerValidationRunRoutes(protected, wired.validations)
	protected.GET("/audit", middleware.RBAC(constants.RoleAuditor, constants.RoleReviewer, constants.RoleAdmin), wired.system.Audit)
	engine.NoRoute(func(context *gin.Context) {
		context.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "route_not_found", "message": "route was not found"}, "request_id": context.GetString("request_id")})
	})
	return engine
}

func wire(db *gorm.DB, cfg config.Config) handlers {
	systemRepository := repository.NewSystemRepository(db)
	cellRepository := repository.NewRobotCellRepository(db)
	zoneRepository := repository.NewSafetyZoneRepository(db)
	programRepository := repository.NewMotionProgramRepository(db)
	validationRepository := repository.NewValidationRunRepository(db)
	systemService := service.NewSystemService(systemRepository, cfg.JWTSecret, cfg.JWTTTL)
	cellService := service.NewRobotCellService(cellRepository, systemService)
	zoneService := service.NewSafetyZoneService(zoneRepository, cellRepository, systemService)
	programService := service.NewMotionProgramService(db, programRepository, cellRepository, systemService)
	validationService := service.NewValidationRunService(db, validationRepository, programRepository, zoneRepository, systemService, cfg.AlgorithmVersion)
	return handlers{
		system: handler.NewSystemHandler(systemService, db), cells: handler.NewRobotCellHandler(cellService),
		zones: handler.NewSafetyZoneHandler(zoneService), programs: handler.NewMotionProgramHandler(programService),
		validations: handler.NewValidationRunHandler(validationService), auth: systemService,
	}
}
