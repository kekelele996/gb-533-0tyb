package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"robot-cell-safety-envelope-validator/backend/internal/dto"
	"robot-cell-safety-envelope-validator/backend/internal/service"
)

type SafetyZoneHandler struct{ service *service.SafetyZoneService }

func NewSafetyZoneHandler(service *service.SafetyZoneService) *SafetyZoneHandler {
	return &SafetyZoneHandler{service: service}
}

func (handler *SafetyZoneHandler) List(context *gin.Context) {
	page, pageSize := Pagination(context)
	items, meta, err := handler.service.List(page, pageSize, QueryUint(context, "robot_cell_id"), context.Query("state"), context.Query("zone_type"))
	if err != nil {
		WriteError(context, err)
		return
	}
	WritePage(context, items, meta)
}

func (handler *SafetyZoneHandler) Get(context *gin.Context) {
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

func (handler *SafetyZoneHandler) Create(context *gin.Context) {
	var request dto.CreateSafetyZoneRequest
	if err := BindAndValidate(context, &request); err != nil {
		WriteError(context, err)
		return
	}
	item, err := handler.service.Create(request, Actor(context), RequestID(context))
	if err != nil {
		WriteError(context, err)
		return
	}
	WriteData(context, http.StatusCreated, item)
}

func (handler *SafetyZoneHandler) Update(context *gin.Context) {
	id, err := PathID(context)
	if err != nil {
		WriteError(context, err)
		return
	}
	var request dto.UpdateSafetyZoneRequest
	if err := BindAndValidate(context, &request); err != nil {
		WriteError(context, err)
		return
	}
	item, err := handler.service.Update(id, request, Actor(context), RequestID(context))
	if err != nil {
		WriteError(context, err)
		return
	}
	WriteData(context, http.StatusOK, item)
}

func (handler *SafetyZoneHandler) Activate(context *gin.Context) { handler.transition(context, true) }
func (handler *SafetyZoneHandler) Deactivate(context *gin.Context) {
	handler.transition(context, false)
}

func (handler *SafetyZoneHandler) transition(context *gin.Context, activate bool) {
	id, err := PathID(context)
	if err != nil {
		WriteError(context, err)
		return
	}
	var request struct {
		Version int `json:"version" validate:"required,gte=1"`
	}
	if err := BindAndValidate(context, &request); err != nil {
		WriteError(context, err)
		return
	}
	var item dto.SafetyZoneResponse
	if activate {
		item, err = handler.service.Activate(id, request.Version, Actor(context), RequestID(context))
	} else {
		item, err = handler.service.Deactivate(id, request.Version, Actor(context), RequestID(context))
	}
	if err != nil {
		WriteError(context, err)
		return
	}
	WriteData(context, http.StatusOK, item)
}
