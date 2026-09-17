package service

import (
	"encoding/json"
	"errors"
	"strings"

	"robot-cell-safety-envelope-validator/backend/internal/constants"
	"robot-cell-safety-envelope-validator/backend/internal/dto"
	"robot-cell-safety-envelope-validator/backend/internal/model"
	"robot-cell-safety-envelope-validator/backend/internal/repository"
)

type RobotCellService struct {
	repository *repository.RobotCellRepository
	system     *SystemService
}

func NewRobotCellService(repository *repository.RobotCellRepository, system *SystemService) *RobotCellService {
	return &RobotCellService{repository: repository, system: system}
}

func (service *RobotCellService) Create(request dto.CreateRobotCellRequest, actor dto.Actor, requestID string) (dto.RobotCellResponse, error) {
	if err := validateLayout(request.LayoutGeoJSON); err != nil {
		return dto.RobotCellResponse{}, Unprocessable("invalid_layout", err.Error(), err)
	}
	cell := model.RobotCell{
		CellCode: strings.ToUpper(strings.TrimSpace(request.CellCode)), Name: strings.TrimSpace(request.Name),
		LayoutGeoJSON: string(request.LayoutGeoJSON), RobotModel: strings.TrimSpace(request.RobotModel),
		ControllerModel: strings.TrimSpace(request.ControllerModel), MaxReachMM: request.MaxReachMM,
		OwnerTeam: strings.TrimSpace(request.OwnerTeam), CellState: constants.CellStateDraft,
		LayoutVersion: 1, CreatedBy: actor.ID,
	}
	if err := service.repository.Create(&cell); err != nil {
		if repository.IsUniqueViolation(err) {
			return dto.RobotCellResponse{}, Conflict("duplicate_cell_code", "cell_code already exists", err)
		}
		return dto.RobotCellResponse{}, Internal("could not create robot cell", err)
	}
	if err := service.system.RecordAudit(actor, requestID, "robot_cell.created", "robot_cell", auditID(cell.ID), map[string]any{"cell_code": cell.CellCode}, nil, cellSummary(cell)); err != nil {
		return dto.RobotCellResponse{}, err
	}
	return service.Get(cell.ID)
}

func (service *RobotCellService) Get(id uint) (dto.RobotCellResponse, error) {
	cell, err := service.repository.Get(id)
	if err != nil {
		return dto.RobotCellResponse{}, MapRepositoryError("robot cell", err)
	}
	return service.response(cell)
}

func (service *RobotCellService) List(page, pageSize int, state, owner string) ([]dto.RobotCellResponse, dto.PageMeta, error) {
	cells, total, err := service.repository.List(page, pageSize, state, owner)
	if err != nil {
		return nil, dto.PageMeta{}, Internal("could not list robot cells", err)
	}
	responses := make([]dto.RobotCellResponse, 0, len(cells))
	for _, cell := range cells {
		response, err := service.response(cell)
		if err != nil {
			return nil, dto.PageMeta{}, err
		}
		responses = append(responses, response)
	}
	return responses, PageMeta(page, pageSize, total), nil
}

func (service *RobotCellService) Update(id uint, request dto.UpdateRobotCellRequest, actor dto.Actor, requestID string) (dto.RobotCellResponse, error) {
	if err := validateLayout(request.LayoutGeoJSON); err != nil {
		return dto.RobotCellResponse{}, Unprocessable("invalid_layout", err.Error(), err)
	}
	before, err := service.repository.Get(id)
	if err != nil {
		return dto.RobotCellResponse{}, MapRepositoryError("robot cell", err)
	}
	updated := before
	updated.Name, updated.LayoutGeoJSON = strings.TrimSpace(request.Name), string(request.LayoutGeoJSON)
	updated.RobotModel, updated.ControllerModel = strings.TrimSpace(request.RobotModel), strings.TrimSpace(request.ControllerModel)
	updated.MaxReachMM, updated.OwnerTeam = request.MaxReachMM, strings.TrimSpace(request.OwnerTeam)
	if err := service.repository.Update(&updated, request.LayoutVersion); err != nil {
		if errors.Is(err, repository.ErrVersionConflict) {
			return dto.RobotCellResponse{}, Conflict("version_conflict", "layout version changed or frozen layouts cannot be edited", err)
		}
		return dto.RobotCellResponse{}, Internal("could not update robot cell", err)
	}
	after, err := service.repository.Get(id)
	if err != nil {
		return dto.RobotCellResponse{}, Internal("could not reload robot cell", err)
	}
	if err := service.system.RecordAudit(actor, requestID, "robot_cell.updated", "robot_cell", auditID(id), map[string]any{"expected_version": request.LayoutVersion}, cellSummary(before), cellSummary(after)); err != nil {
		return dto.RobotCellResponse{}, err
	}
	return service.response(after)
}

func (service *RobotCellService) Freeze(id uint, actor dto.Actor, requestID string) (dto.RobotCellResponse, error) {
	return service.transition(id, constants.CellStateDraft, constants.CellStateFrozen, "robot_cell.layout_frozen", actor, requestID)
}

func (service *RobotCellService) Deactivate(id uint, actor dto.Actor, requestID string) (dto.RobotCellResponse, error) {
	before, err := service.repository.Get(id)
	if err != nil {
		return dto.RobotCellResponse{}, MapRepositoryError("robot cell", err)
	}
	if before.CellState == constants.CellStateInactive {
		return dto.RobotCellResponse{}, Conflict("state_conflict", "robot cell is already inactive", repository.ErrStateConflict)
	}
	return service.transition(id, before.CellState, constants.CellStateInactive, "robot_cell.deactivated", actor, requestID)
}

func (service *RobotCellService) transition(id uint, from, to, action string, actor dto.Actor, requestID string) (dto.RobotCellResponse, error) {
	before, err := service.repository.Get(id)
	if err != nil {
		return dto.RobotCellResponse{}, MapRepositoryError("robot cell", err)
	}
	if before.CellState != from {
		return dto.RobotCellResponse{}, Conflict("state_conflict", "robot cell state no longer allows this action", repository.ErrStateConflict)
	}
	if err := service.repository.Transition(id, from, to); err != nil {
		return dto.RobotCellResponse{}, Conflict("state_conflict", "robot cell state changed concurrently", err)
	}
	after, err := service.repository.Get(id)
	if err != nil {
		return dto.RobotCellResponse{}, Internal("could not reload robot cell", err)
	}
	if err := service.system.RecordAudit(actor, requestID, action, "robot_cell", auditID(id), nil, cellSummary(before), cellSummary(after)); err != nil {
		return dto.RobotCellResponse{}, err
	}
	return service.response(after)
}

func (service *RobotCellService) response(cell model.RobotCell) (dto.RobotCellResponse, error) {
	zones, programs, err := service.repository.Counts(cell.ID)
	if err != nil {
		return dto.RobotCellResponse{}, Internal("could not summarize robot cell", err)
	}
	return dto.RobotCellResponse{
		ID: cell.ID, CellCode: cell.CellCode, Name: cell.Name, LayoutGeoJSON: json.RawMessage(cell.LayoutGeoJSON),
		RobotModel: cell.RobotModel, ControllerModel: cell.ControllerModel, MaxReachMM: cell.MaxReachMM,
		OwnerTeam: cell.OwnerTeam, CellState: cell.CellState, LayoutVersion: cell.LayoutVersion,
		ZoneCount: zones, ProgramCount: programs, CreatedAt: cell.CreatedAt, UpdatedAt: cell.UpdatedAt,
	}, nil
}

func validateLayout(raw []byte) error {
	var document map[string]interface{}
	if err := json.Unmarshal(raw, &document); err != nil {
		return errors.New("layout_geojson must be valid JSON")
	}
	typeName, _ := document["type"].(string)
	if typeName != "FeatureCollection" && typeName != "Polygon" && typeName != "Feature" {
		return errors.New("layout_geojson type must be FeatureCollection, Feature, or Polygon")
	}
	return nil
}

func cellSummary(cell model.RobotCell) map[string]any {
	return map[string]any{"cell_code": cell.CellCode, "cell_state": cell.CellState, "layout_version": cell.LayoutVersion, "owner_team": cell.OwnerTeam}
}
