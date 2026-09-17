package repository

import (
	"fmt"

	"gorm.io/gorm"

	"robot-cell-safety-envelope-validator/backend/internal/model"
)

type SafetyZoneRepository struct{ db *gorm.DB }

func NewSafetyZoneRepository(db *gorm.DB) *SafetyZoneRepository { return &SafetyZoneRepository{db: db} }
func (repository *SafetyZoneRepository) WithDB(db *gorm.DB) *SafetyZoneRepository {
	return &SafetyZoneRepository{db: db}
}

func (repository *SafetyZoneRepository) Create(zone *model.SafetyZone) error {
	if err := repository.db.Create(zone).Error; err != nil {
		return fmt.Errorf("create safety zone: %w", err)
	}
	return nil
}

func (repository *SafetyZoneRepository) Get(id uint) (model.SafetyZone, error) {
	var zone model.SafetyZone
	if err := repository.db.Preload("RobotCell").First(&zone, id).Error; err != nil {
		return zone, fmt.Errorf("get safety zone: %w", err)
	}
	return zone, nil
}

func (repository *SafetyZoneRepository) List(page, pageSize int, cellID uint, state, zoneType string) ([]model.SafetyZone, int64, error) {
	query := repository.db.Model(&model.SafetyZone{})
	if cellID > 0 {
		query = query.Where("robot_cell_id = ?", cellID)
	}
	if state != "" {
		query = query.Where("zone_state = ?", state)
	}
	if zoneType != "" {
		query = query.Where("zone_type = ?", zoneType)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count safety zones: %w", err)
	}
	var zones []model.SafetyZone
	if err := query.Preload("RobotCell").Order("robot_cell_id ASC, name ASC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&zones).Error; err != nil {
		return nil, 0, fmt.Errorf("list safety zones: %w", err)
	}
	return zones, total, nil
}

func (repository *SafetyZoneRepository) ActiveForCell(cellID uint) ([]model.SafetyZone, error) {
	var zones []model.SafetyZone
	if err := repository.db.Where("robot_cell_id = ? AND zone_state = ?", cellID, "active").Order("id ASC").Find(&zones).Error; err != nil {
		return nil, fmt.Errorf("list active safety zones: %w", err)
	}
	return zones, nil
}

func (repository *SafetyZoneRepository) Update(zone *model.SafetyZone, expectedVersion int) error {
	result := repository.db.Model(&model.SafetyZone{}).
		Where("id = ? AND version = ? AND zone_state <> ?", zone.ID, expectedVersion, "inactive").
		Updates(map[string]any{
			"name": zone.Name, "zone_type": zone.ZoneType, "polygon_geo_json": zone.PolygonGeoJSON,
			"min_height_mm": zone.MinHeightMM, "max_height_mm": zone.MaxHeightMM,
			"speed_limit_mm_s": zone.SpeedLimitMMS, "access_rule": zone.AccessRule,
			"version": expectedVersion + 1, "zone_state": "draft",
		})
	if result.Error != nil {
		return fmt.Errorf("update safety zone: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrVersionConflict
	}
	return nil
}

func (repository *SafetyZoneRepository) Transition(id uint, version int, from, to string) error {
	result := repository.db.Model(&model.SafetyZone{}).
		Where("id = ? AND version = ? AND zone_state = ?", id, version, from).
		Updates(map[string]any{"zone_state": to, "version": version + 1})
	if result.Error != nil {
		return fmt.Errorf("transition safety zone: %w", result.Error)
	}
	if result.RowsAffected != 1 {
		return ErrStateConflict
	}
	return nil
}
