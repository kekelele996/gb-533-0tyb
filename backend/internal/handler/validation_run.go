package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"robot-cell-safety-envelope-validator/backend/internal/dto"
	"robot-cell-safety-envelope-validator/backend/internal/service"
)

type ValidationRunHandler struct{ service *service.ValidationRunService }

func NewValidationRunHandler(service *service.ValidationRunService) *ValidationRunHandler {
	return &ValidationRunHandler{service: service}
}

func (handler *ValidationRunHandler) List(context *gin.Context) {
	page, pageSize := Pagination(context)
	items, meta, err := handler.service.List(page, pageSize, QueryUint(context, "motion_program_id"), context.Query("status"))
	if err != nil {
		WriteError(context, err)
		return
	}
	WritePage(context, items, meta)
}

func (handler *ValidationRunHandler) Get(context *gin.Context) {
	id, err := PathID(context)
	if err != nil {
		WriteError(context, err)
		return
	}
	item, err := handler.service.Get(id)
	if err != nil {
		WriteError(context, err)
		return
	}
	WriteData(context, http.StatusOK, item)
}

func (handler *ValidationRunHandler) Create(context *gin.Context) {
	var request dto.CreateValidationRunRequest
	if err := BindAndValidate(context, &request); err != nil {
		WriteError(context, err)
		return
	}
	item, reused, err := handler.service.Create(request, context.GetHeader("Idempotency-Key"), Actor(context), RequestID(context))
	if err != nil {
		WriteError(context, err)
		return
	}
	status := http.StatusCreated
	if reused {
		status = http.StatusOK
	}
	WriteData(context, status, item)
}

func (handler *ValidationRunHandler) Review(context *gin.Context) {
	handler.reviewAction(context, handler.service.Review)
}
func (handler *ValidationRunHandler) Accept(context *gin.Context) {
	handler.reviewAction(context, handler.service.Accept)
}
func (handler *ValidationRunHandler) Void(context *gin.Context) {
	handler.reviewAction(context, handler.service.Void)
}

func (handler *ValidationRunHandler) reviewAction(context *gin.Context, action func(uint, string, dto.Actor, string) (dto.ValidationRunResponse, error)) {
	id, err := PathID(context)
	if err != nil {
		WriteError(context, err)
		return
	}
	var request dto.ReviewValidationRequest
	if err := BindAndValidate(context, &request); err != nil {
		WriteError(context, err)
		return
	}
	item, err := action(id, request.Note, Actor(context), RequestID(context))
	if err != nil {
		WriteError(context, err)
		return
	}
	WriteData(context, http.StatusOK, item)
}
