package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"

	"robot-cell-safety-envelope-validator/backend/internal/dto"
	"robot-cell-safety-envelope-validator/backend/internal/service"
)

const ActorContextKey = "authenticated_actor"

var validate = validator.New()

type SystemHandler struct {
	service *service.SystemService
	db      *gorm.DB
}

func NewSystemHandler(service *service.SystemService, db *gorm.DB) *SystemHandler {
	return &SystemHandler{service: service, db: db}
}

func (handler *SystemHandler) Login(context *gin.Context) {
	var request dto.LoginRequest
	if err := BindAndValidate(context, &request); err != nil {
		WriteError(context, err)
		return
	}
	response, err := handler.service.Login(request)
	if err != nil {
		WriteError(context, err)
		return
	}
	WriteData(context, http.StatusOK, response)
}

func (handler *SystemHandler) Health(context *gin.Context) {
	WriteData(context, http.StatusOK, gin.H{"status": "ok", "service": "robot-cell-safety-envelope-validator", "time": time.Now().UTC()})
}

func (handler *SystemHandler) Ready(context *gin.Context) {
	sqlDB, err := handler.db.DB()
	if err != nil {
		WriteError(context, service.Internal("database handle is unavailable", err))
		return
	}
	if err := sqlDB.PingContext(context.Request.Context()); err != nil {
		WriteError(context, service.Internal("database is unavailable", err))
		return
	}
	WriteData(context, http.StatusOK, gin.H{"status": "ready", "database": "available"})
}

func (handler *SystemHandler) Audit(context *gin.Context) {
	page, pageSize := Pagination(context)
	from, err := optionalTime(context.Query("from"))
	if err != nil {
		WriteError(context, service.BadRequest("invalid_time_filter", "from must be RFC3339", err))
		return
	}
	to, err := optionalTime(context.Query("to"))
	if err != nil {
		WriteError(context, service.BadRequest("invalid_time_filter", "to must be RFC3339", err))
		return
	}
	events, meta, err := handler.service.ListAudit(page, pageSize, context.Query("actor"), context.Query("request_id"), context.Query("resource_type"), context.Query("action"), from, to)
	if err != nil {
		WriteError(context, err)
		return
	}
	WritePage(context, events, meta)
}

func BindAndValidate(context *gin.Context, target any) error {
	if err := context.ShouldBindJSON(target); err != nil {
		return service.BadRequest("invalid_json", "request body is not valid JSON", err)
	}
	if err := validate.Struct(target); err != nil {
		return service.BadRequest("validation_error", validationMessage(err), err)
	}
	return nil
}

func WriteData(context *gin.Context, status int, data any) {
	context.JSON(status, gin.H{"data": data, "request_id": RequestID(context)})
}

func WritePage(context *gin.Context, data any, meta dto.PageMeta) {
	context.JSON(http.StatusOK, gin.H{"data": data, "meta": meta, "request_id": RequestID(context)})
}

func WriteError(context *gin.Context, err error) {
	appError := &service.AppError{}
	if !errors.As(err, &appError) {
		appError = service.Internal("unexpected server error", err)
	}
	if appError.Status >= 500 {
		slog.Error("request failed", "request_id", RequestID(context), "code", appError.Code, "error", appError.Error())
	}
	context.AbortWithStatusJSON(appError.Status, gin.H{"error": gin.H{"code": appError.Code, "message": appError.Message}, "request_id": RequestID(context)})
}

func Actor(context *gin.Context) dto.Actor {
	actor, _ := context.MustGet(ActorContextKey).(dto.Actor)
	return actor
}

func RequestID(context *gin.Context) string { return context.GetString("request_id") }

func Pagination(context *gin.Context) (int, int) {
	page, _ := strconv.Atoi(context.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(context.DefaultQuery("page_size", "50"))
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}
	return page, pageSize
}

func PathID(context *gin.Context) (uint, error) {
	value, err := strconv.ParseUint(context.Param("id"), 10, 32)
	if err != nil || value == 0 {
		return 0, service.BadRequest("invalid_id", "path id must be a positive integer")
	}
	return uint(value), nil
}

func QueryUint(context *gin.Context, key string) uint {
	value, _ := strconv.ParseUint(context.Query(key), 10, 32)
	return uint(value)
}

func optionalTime(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	return &parsed, err
}

func validationMessage(err error) string {
	var validationErrors validator.ValidationErrors
	if !errors.As(err, &validationErrors) || len(validationErrors) == 0 {
		return "request fields are invalid"
	}
	item := validationErrors[0]
	return fmt.Sprintf("field %s failed %s validation", item.Field(), item.Tag())
}
