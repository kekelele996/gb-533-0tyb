package repository

import (
	"fmt"

	"gorm.io/gorm"

	"robot-cell-safety-envelope-validator/backend/internal/model"
)

type RobotCellRepository struct{ db *gorm.DB }

func NewRobotCellRepository(db *gorm.DB) *RobotCellRepository { return &RobotCellRepository{db: db} }
func (repository *RobotCellRepository) WithDB(db *gorm.DB) *RobotCellRepository {
	return &RobotCellRepository{db: db}
}

func (repository *RobotCellRepository) Create(cell *model.RobotCell) error {
	if err := repository.db.Create(cell).Error; err != nil {
		return fmt.Errorf("create robot cell: %w", err)
	}
	return nil
}

func (repository *RobotCellRepository) Get(id uint) (model.RobotCell, error) {
	var cell model.RobotCell
	if err := repository.db.First(&cell, id).Error; err != nil {
		return cell, fmt.Errorf("get robot cell: %w", err)
	}
	return cell, nil
}

func (repository *RobotCellRepository) List(page, pageSize int, state, owner string) ([]model.RobotCell, int64, error) {
	query := repository.db.Model(&model.RobotCell{})
	if state != "" {
		query = query.Where("cell_state = ?", state)
	}
	if owner != "" {
		query = query.Where("owner_team = ?", owner)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count robot cells: %w", err)
	}
	var cells []model.RobotCell
	if err := query.Order("cell_code ASC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&cells).Error; err != nil {
		return nil, 0, fmt.Errorf("list robot cells: %w", err)
	}
	return cells, total, nil
}

func (repository *RobotCellRepository) Update(cell *model.RobotCell, expectedVersion int) error {
	result := repository.db.Model(&model.RobotCell{}).
		Where("id = ? AND layout_version = ? AND cell_state = ?", cell.ID, expectedVersion, "draft").
		Updates(map[string]any{
			"name": cell.Name, "layout_geo_json": cell.LayoutGeoJSON, "robot_model": cell.RobotModel,
			"controller_model": cell.ControllerModel, "max_reach_mm": cell.MaxReachMM,
			"owner_team": cell.OwnerTeam, "layout_version": expectedVersion + 1,
		})
	if result.Error != nil {
		return fmt.Errorf("update robot cell: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrVersionConflict
	}
	return nil
}

func (repository *RobotCellRepository) Transition(id uint, from, to string) error {
	result := repository.db.Model(&model.RobotCell{}).Where("id = ? AND cell_state = ?", id, from).Update("cell_state", to)
	if result.Error != nil {
		return fmt.Errorf("transition robot cell: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrStateConflict
	}
	return nil
}

func (repository *RobotCellRepository) Counts(id uint) (int64, int64, error) {
	var zones, programs int64
	if err := repository.db.Model(&model.SafetyZone{}).Where("robot_cell_id = ?", id).Count(&zones).Error; err != nil {
		return 0, 0, fmt.Errorf("count zones: %w", err)
	}
	if err := repository.db.Model(&model.MotionProgram{}).Where("robot_cell_id = ?", id).Count(&programs).Error; err != nil {
		return 0, 0, fmt.Errorf("count programs: %w", err)
	}
	return zones, programs, nil
}
