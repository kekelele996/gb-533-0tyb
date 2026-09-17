package model

import "time"

type SafetyZone struct {
	ID             uint      `gorm:"primaryKey"`
	RobotCellID    uint      `gorm:"index:idx_zone_cell_name,unique;not null"`
	Name           string    `gorm:"size:120;index:idx_zone_cell_name,unique;not null"`
	ZoneType       string    `gorm:"size:24;index;not null"`
	PolygonGeoJSON string    `gorm:"type:text;not null"`
	MinHeightMM    float64   `gorm:"not null"`
	MaxHeightMM    float64   `gorm:"not null"`
	SpeedLimitMMS  float64   `gorm:"column:speed_limit_mm_s;not null"`
	AccessRule     string    `gorm:"size:300;not null"`
	ZoneState      string    `gorm:"size:24;index;not null"`
	Version        int       `gorm:"not null;default:1"`
	CreatedBy      uint      `gorm:"index;not null"`
	CreatedAt      time.Time `gorm:"not null"`
	UpdatedAt      time.Time `gorm:"not null"`
	RobotCell      RobotCell `gorm:"foreignKey:RobotCellID"`
}
