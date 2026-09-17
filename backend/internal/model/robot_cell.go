package model

import "time"

type RobotCell struct {
	ID              uint      `gorm:"primaryKey"`
	CellCode        string    `gorm:"size:64;uniqueIndex;not null"`
	Name            string    `gorm:"size:160;not null"`
	LayoutGeoJSON   string    `gorm:"type:text;not null"`
	RobotModel      string    `gorm:"size:120;not null"`
	ControllerModel string    `gorm:"size:120;not null"`
	MaxReachMM      float64   `gorm:"not null"`
	OwnerTeam       string    `gorm:"size:120;index;not null"`
	CellState       string    `gorm:"size:24;index;not null"`
	LayoutVersion   int       `gorm:"not null;default:1"`
	CreatedBy       uint      `gorm:"index;not null"`
	CreatedAt       time.Time `gorm:"not null"`
	UpdatedAt       time.Time `gorm:"not null"`
}
