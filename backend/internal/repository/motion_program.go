package repository

import (
	"fmt"

	"gorm.io/gorm"

	"robot-cell-safety-envelope-validator/backend/internal/model"
)

type MotionProgramRepository struct{ db *gorm.DB }

func NewMotionProgramRepository(db *gorm.DB) *MotionProgramRepository {
	return &MotionProgramRepository{db: db}
}
func (repository *MotionProgramRepository) WithDB(db *gorm.DB) *MotionProgramRepository {
	return &MotionProgramRepository{db: db}
}

func (repository *MotionProgramRepository) Create(program *model.MotionProgram) error {
	if err := repository.db.Create(program).Error; err != nil {
		return fmt.Errorf("create motion program: %w", err)
	}
	return nil
}

func (repository *MotionProgramRepository) Get(id uint) (model.MotionProgram, error) {
	var program model.MotionProgram
	if err := repository.db.Preload("RobotCell").First(&program, id).Error; err != nil {
		return program, fmt.Errorf("get motion program: %w", err)
	}
	return program, nil
}

func (repository *MotionProgramRepository) List(page, pageSize int, cellID uint, state string) ([]model.MotionProgram, int64, error) {
	query := repository.db.Model(&model.MotionProgram{})
	if cellID > 0 {
		query = query.Where("robot_cell_id = ?", cellID)
	}
	if state != "" {
		query = query.Where("program_state = ?", state)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count motion programs: %w", err)
	}
	var programs []model.MotionProgram
	if err := query.Preload("RobotCell").Order("program_code ASC, version DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&programs).Error; err != nil {
		return nil, 0, fmt.Errorf("list motion programs: %w", err)
	}
	return programs, total, nil
}

func (repository *MotionProgramRepository) Transition(id uint, from, to string) error {
	result := repository.db.Model(&model.MotionProgram{}).Where("id = ? AND program_state = ?", id, from).Update("program_state", to)
	if result.Error != nil {
		return fmt.Errorf("transition motion program: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrStateConflict
	}
	return nil
}

func (repository *MotionProgramRepository) SupersedeActive(cellID, exceptID uint) error {
	if err := repository.db.Model(&model.MotionProgram{}).
		Where("robot_cell_id = ? AND id <> ? AND program_state = ?", cellID, exceptID, "active").
		Update("program_state", "superseded").Error; err != nil {
		return fmt.Errorf("supersede active programs: %w", err)
	}
	return nil
}
