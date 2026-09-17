package repository

import (
	"fmt"

	"gorm.io/gorm"

	"robot-cell-safety-envelope-validator/backend/internal/model"
)

type ValidationRunRepository struct{ db *gorm.DB }

func NewValidationRunRepository(db *gorm.DB) *ValidationRunRepository {
	return &ValidationRunRepository{db: db}
}
func (repository *ValidationRunRepository) WithDB(db *gorm.DB) *ValidationRunRepository {
	return &ValidationRunRepository{db: db}
}

func (repository *ValidationRunRepository) Create(run *model.ValidationRun) error {
	if err := repository.db.Create(run).Error; err != nil {
		return fmt.Errorf("create validation run: %w", err)
	}
	return nil
}

func (repository *ValidationRunRepository) Get(id uint) (model.ValidationRun, error) {
	var run model.ValidationRun
	if err := repository.db.Preload("MotionProgram").First(&run, id).Error; err != nil {
		return run, fmt.Errorf("get validation run: %w", err)
	}
	return run, nil
}

func (repository *ValidationRunRepository) List(page, pageSize int, programID uint, status string) ([]model.ValidationRun, int64, error) {
	query := repository.db.Model(&model.ValidationRun{})
	if programID > 0 {
		query = query.Where("motion_program_id = ?", programID)
	}
	if status != "" {
		query = query.Where("validation_status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count validation runs: %w", err)
	}
	var runs []model.ValidationRun
	if err := query.Preload("MotionProgram").Order("started_at DESC, id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&runs).Error; err != nil {
		return nil, 0, fmt.Errorf("list validation runs: %w", err)
	}
	return runs, total, nil
}

func (repository *ValidationRunRepository) FindByIdempotencyKey(key string) (model.ValidationRun, error) {
	var run model.ValidationRun
	if err := repository.db.Preload("MotionProgram").Where("idempotency_key = ?", key).First(&run).Error; err != nil {
		return run, fmt.Errorf("find idempotent run: %w", err)
	}
	return run, nil
}

func (repository *ValidationRunRepository) LatestByInput(inputHash, algorithmVersion string) (model.ValidationRun, error) {
	var run model.ValidationRun
	if err := repository.db.Preload("MotionProgram").Where("input_hash = ? AND algorithm_version = ?", inputHash, algorithmVersion).
		Order("attempt DESC, id DESC").First(&run).Error; err != nil {
		return run, fmt.Errorf("find latest input run: %w", err)
	}
	return run, nil
}

func (repository *ValidationRunRepository) SetSimulating(id uint) error {
	result := repository.db.Model(&model.ValidationRun{}).Where("id = ? AND validation_status = ?", id, "queued").Update("validation_status", "simulating")
	if result.Error != nil {
		return fmt.Errorf("start validation run: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrStateConflict
	}
	return nil
}

func (repository *ValidationRunRepository) Finish(run *model.ValidationRun) error {
	result := repository.db.Model(&model.ValidationRun{}).Where("id = ? AND validation_status = ?", run.ID, "simulating").Updates(map[string]any{
		"collision_events_json": run.CollisionEventsJSON, "interlock_findings_json": run.InterlockFindingsJSON,
		"risk_score": run.RiskScore, "validation_status": run.ValidationStatus,
		"explanation": run.Explanation, "finished_at": run.FinishedAt,
	})
	if result.Error != nil {
		return fmt.Errorf("finish validation run: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrStateConflict
	}
	return nil
}

func (repository *ValidationRunRepository) Review(id uint, from, to string, reviewer uint, note string) error {
	result := repository.db.Model(&model.ValidationRun{}).Where("id = ? AND validation_status = ?", id, from).
		Updates(map[string]any{"validation_status": to, "reviewed_by": reviewer, "reviewed_at": gorm.Expr("CURRENT_TIMESTAMP"), "review_note": note})
	if result.Error != nil {
		return fmt.Errorf("review validation run: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrStateConflict
	}
	return nil
}
