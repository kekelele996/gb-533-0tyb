package service

import (
	"encoding/json"
	"errors"
	"strings"

	"robot-cell-safety-envelope-validator/backend/internal/constants"
	"robot-cell-safety-envelope-validator/backend/internal/dto"
	"robot-cell-safety-envelope-validator/backend/internal/geometry"
	"robot-cell-safety-envelope-validator/backend/internal/model"
	"robot-cell-safety-envelope-validator/backend/internal/repository"
)

type SafetyZoneService struct {
	repository *repository.SafetyZoneRepository
	cells      *repository.RobotCellRepository
	system     *SystemService
}

func NewSafetyZoneService(repository *repository.SafetyZoneRepository, cells *repository.RobotCellRepository, system *SystemService) *SafetyZoneService {
	return &SafetyZoneService{repository: repository, cells: cells, system: system}
}

func (service *SafetyZoneService) Create(request dto.CreateSafetyZoneRequest, actor dto.Actor, requestID string) (dto.SafetyZoneResponse, error) {
	if err := validateZone(request.ZoneType, request.PolygonGeoJSON, request.MinHeightMM, request.MaxHeightMM, request.SpeedLimitMMS); err != nil {
		return dto.SafetyZoneResponse{}, err
	}
	cell, err := service.cells.Get(request.RobotCellID)
	if err != nil {
		return dto.SafetyZoneResponse{}, MapRepositoryError("robot cell", err)
	}
	if cell.CellState == constants.CellStateInactive {
		return dto.SafetyZoneResponse{}, Conflict("cell_inactive", "cannot add a zone to an inactive cell", repository.ErrStateConflict)
	}
	zone := model.SafetyZone{
		RobotCellID: request.RobotCellID, Name: strings.TrimSpace(request.Name), ZoneType: request.ZoneType,
		PolygonGeoJSON: string(request.PolygonGeoJSON), MinHeightMM: request.MinHeightMM, MaxHeightMM: request.MaxHeightMM,
		SpeedLimitMMS: request.SpeedLimitMMS, AccessRule: strings.TrimSpace(request.AccessRule),
		ZoneState: constants.ZoneStateDraft, Version: 1, CreatedBy: actor.ID,
	}
	if err := service.repository.Create(&zone); err != nil {
		if repository.IsUniqueViolation(err) {
			return dto.SafetyZoneResponse{}, Conflict("duplicate_zone_name", "zone name already exists in this cell", err)
		}
		return dto.SafetyZoneResponse{}, Internal("could not create safety zone", err)
	}
	if err := service.system.RecordAudit(actor, requestID, "safety_zone.created", "safety_zone", auditID(zone.ID), map[string]any{"robot_cell_id": zone.RobotCellID}, nil, zoneSummary(zone)); err != nil {
		return dto.SafetyZoneResponse{}, err
	}
	return service.Get(zone.ID)
}

func (service *SafetyZoneService) Get(id uint) (dto.SafetyZoneResponse, error) {
	zone, err := service.repository.Get(id)
	if err != nil {
		return dto.SafetyZoneResponse{}, MapRepositoryError("safety zone", err)
	}
	return zoneResponse(zone), nil
}

func (service *SafetyZoneService) List(page, pageSize int, cellID uint, state, zoneType string) ([]dto.SafetyZoneResponse, dto.PageMeta, error) {
	zones, total, err := service.repository.List(page, pageSize, cellID, state, zoneType)
	if err != nil {
		return nil, dto.PageMeta{}, Internal("could not list safety zones", err)
	}
	responses := make([]dto.SafetyZoneResponse, 0, len(zones))
	for _, zone := range zones {
		responses = append(responses, zoneResponse(zone))
	}
	return responses, PageMeta(page, pageSize, total), nil
}

func (service *SafetyZoneService) Update(id uint, request dto.UpdateSafetyZoneRequest, actor dto.Actor, requestID string) (dto.SafetyZoneResponse, error) {
	if err := validateZone(request.ZoneType, request.PolygonGeoJSON, request.MinHeightMM, request.MaxHeightMM, request.SpeedLimitMMS); err != nil {
		return dto.SafetyZoneResponse{}, err
	}
	before, err := service.repository.Get(id)
	if err != nil {
		return dto.SafetyZoneResponse{}, MapRepositoryError("safety zone", err)
	}
	updated := before
	updated.Name, updated.ZoneType, updated.PolygonGeoJSON = strings.TrimSpace(request.Name), request.ZoneType, string(request.PolygonGeoJSON)
	updated.MinHeightMM, updated.MaxHeightMM, updated.SpeedLimitMMS = request.MinHeightMM, request.MaxHeightMM, request.SpeedLimitMMS
	updated.AccessRule = strings.TrimSpace(request.AccessRule)
	if err := service.repository.Update(&updated, request.Version); err != nil {
		if errors.Is(err, repository.ErrVersionConflict) {
			return dto.SafetyZoneResponse{}, Conflict("version_conflict", "zone version changed or the zone is inactive", err)
		}
		if repository.IsUniqueViolation(err) {
			return dto.SafetyZoneResponse{}, Conflict("duplicate_zone_name", "zone name already exists in this cell", err)
		}
		return dto.SafetyZoneResponse{}, Internal("could not update safety zone", err)
	}
	after, err := service.repository.Get(id)
	if err != nil {
		return dto.SafetyZoneResponse{}, Internal("could not reload safety zone", err)
	}
	if err := service.system.RecordAudit(actor, requestID, "safety_zone.revised", "safety_zone", auditID(id), map[string]any{"expected_version": request.Version}, zoneSummary(before), zoneSummary(after)); err != nil {
		return dto.SafetyZoneResponse{}, err
	}
	return zoneResponse(after), nil
}

func (service *SafetyZoneService) Activate(id uint, version int, actor dto.Actor, requestID string) (dto.SafetyZoneResponse, error) {
	zone, err := service.repository.Get(id)
	if err != nil {
		return dto.SafetyZoneResponse{}, MapRepositoryError("safety zone", err)
	}
	if zone.ZoneState != constants.ZoneStateDraft {
		return dto.SafetyZoneResponse{}, Conflict("state_conflict", "only draft zones can be activated", repository.ErrStateConflict)
	}
	cell, err := service.cells.Get(zone.RobotCellID)
	if err != nil {
		return dto.SafetyZoneResponse{}, MapRepositoryError("robot cell", err)
	}
	if cell.CellState == constants.CellStateInactive {
		return dto.SafetyZoneResponse{}, Conflict("cell_inactive", "zones in an inactive cell cannot be activated", repository.ErrStateConflict)
	}
	return service.transition(zone, version, constants.ZoneStateActive, "safety_zone.activated", actor, requestID)
}

func (service *SafetyZoneService) Deactivate(id uint, version int, actor dto.Actor, requestID string) (dto.SafetyZoneResponse, error) {
	zone, err := service.repository.Get(id)
	if err != nil {
		return dto.SafetyZoneResponse{}, MapRepositoryError("safety zone", err)
	}
	if zone.ZoneState == constants.ZoneStateInactive {
		return dto.SafetyZoneResponse{}, Conflict("state_conflict", "safety zone is already inactive", repository.ErrStateConflict)
	}
	return service.transition(zone, version, constants.ZoneStateInactive, "safety_zone.deactivated", actor, requestID)
}

func (service *SafetyZoneService) transition(before model.SafetyZone, version int, target, action string, actor dto.Actor, requestID string) (dto.SafetyZoneResponse, error) {
	if before.Version != version {
		return dto.SafetyZoneResponse{}, Conflict("version_conflict", "zone version changed", repository.ErrVersionConflict)
	}
	if err := service.repository.Transition(before.ID, version, before.ZoneState, target); err != nil {
		return dto.SafetyZoneResponse{}, Conflict("state_conflict", "zone state or version changed concurrently", err)
	}
	after, err := service.repository.Get(before.ID)
	if err != nil {
		return dto.SafetyZoneResponse{}, Internal("could not reload safety zone", err)
	}
	if err := service.system.RecordAudit(actor, requestID, action, "safety_zone", auditID(before.ID), map[string]any{"expected_version": version}, zoneSummary(before), zoneSummary(after)); err != nil {
		return dto.SafetyZoneResponse{}, err
	}
	return zoneResponse(after), nil
}

func validateZone(zoneType string, polygon []byte, minHeight, maxHeight, speed float64) error {
	if !constants.ValidZoneType(zoneType) {
		return Unprocessable("invalid_zone_type", "zone_type must be operating, restricted, service, or escape", nil)
	}
	if maxHeight <= minHeight {
		return Unprocessable("invalid_height_interval", "max_height_mm must be greater than min_height_mm", nil)
	}
	if speed < 0 {
		return Unprocessable("invalid_speed_limit", "speed_limit_mm_s cannot be negative", nil)
	}
	if _, err := geometry.ParsePolygon(polygon); err != nil {
		return Unprocessable("invalid_geometry", err.Error(), err)
	}
	return nil
}

func zoneResponse(zone model.SafetyZone) dto.SafetyZoneResponse {
	return dto.SafetyZoneResponse{
		ID: zone.ID, RobotCellID: zone.RobotCellID, RobotCellCode: zone.RobotCell.CellCode,
		Name: zone.Name, ZoneType: zone.ZoneType, PolygonGeoJSON: json.RawMessage(zone.PolygonGeoJSON),
		MinHeightMM: zone.MinHeightMM, MaxHeightMM: zone.MaxHeightMM, SpeedLimitMMS: zone.SpeedLimitMMS,
		AccessRule: zone.AccessRule, ZoneState: zone.ZoneState, Version: zone.Version,
		CreatedAt: zone.CreatedAt, UpdatedAt: zone.UpdatedAt,
	}
}

func zoneSummary(zone model.SafetyZone) map[string]any {
	return map[string]any{"name": zone.Name, "zone_type": zone.ZoneType, "zone_state": zone.ZoneState, "version": zone.Version, "robot_cell_id": zone.RobotCellID}
}
