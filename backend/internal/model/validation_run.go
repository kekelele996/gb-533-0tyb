package model

import "time"

type ValidationRun struct {
	ID                    uint          `gorm:"primaryKey"`
	MotionProgramID       uint          `gorm:"index;not null"`
	ZoneSnapshot          string        `gorm:"type:text;not null"`
	ProgramSnapshot       string        `gorm:"type:text;not null"`
	AlgorithmVersion      string        `gorm:"size:80;index;not null"`
	InputHash             string        `gorm:"size:64;index;not null"`
	IdempotencyKey        string        `gorm:"size:120;uniqueIndex;not null"`
	Attempt               int           `gorm:"not null;default:1"`
	RetryOfID             *uint         `gorm:"index"`
	CollisionEventsJSON   string        `gorm:"type:text;not null"`
	InterlockFindingsJSON string        `gorm:"type:text;not null"`
	RiskScore             float64       `gorm:"not null"`
	ValidationStatus      string        `gorm:"size:24;index;not null"`
	Explanation           string        `gorm:"type:text;not null"`
	RequestedBy           uint          `gorm:"index;not null"`
	StartedAt             time.Time     `gorm:"index;not null"`
	FinishedAt            *time.Time    `gorm:"index"`
	ReviewedBy            *uint         `gorm:"index"`
	ReviewedAt            *time.Time    `gorm:"index"`
	ReviewNote            string        `gorm:"type:text;not null"`
	MotionProgram         MotionProgram `gorm:"foreignKey:MotionProgramID"`
}
